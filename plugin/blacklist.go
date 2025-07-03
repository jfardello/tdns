package plugin

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/jfardello/tdns/sched"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"strings"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/log"

	"github.com/armon/go-radix"
	"github.com/miekg/dns"

	"github.com/dustin/go-humanize"
	"github.com/go-git/go-git/v5"
	gitconf "github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/sirupsen/logrus"
)

const (
	BLAddr         string = "0.0.0.0"
	BLMaxFuzziness        = 300
)

type None struct{}

type WriteCounter struct {
	Total    uint64
	ReportMB int
	logger   *logrus.Entry
}

func (wc *WriteCounter) Write(p []byte) (int, error) {
	n := len(p)
	wc.Total += uint64(n)
	if wc.Total >= uint64(1024*1024*wc.ReportMB) {
		wc.PrintProgress()
		wc.ReportMB++
	}
	return n, nil
}

func (wc *WriteCounter) PrintProgress() {
	wc.logger.Infof("Downloaded %s", humanize.Bytes(wc.Total))
}

type BlackListPlugin struct {
	Enabled      bool
	Hole         *radix.Tree
	HoleFile     string
	externalFile string
	externalRepo string
	branch       string
	WhiteList    []string
	DefaultAddr  string
	ctx          context.Context
	cancel       context.CancelFunc
}

// Info Ptype indicates where to hook the plugin.
func (bp *BlackListPlugin) Info() (string, Ptype) {
	return "blacklist", PreRouting
}

func (bp *BlackListPlugin) Run(mess *Message) (message *Message, err error) {
	logger := log.GetLogger("plugin", "BHole")
	if !bp.Enabled {
		logger.Debug("Blackhole disabled.")
		return mess, nil
	}
	m, err := mess.GetMsg()
	if err != nil {
		return mess, err
	}
	domain := strings.TrimSuffix(m.Question[0].Name, ".")

	_, ok := bp.Hole.Get(domain)
	if ok {
		logger.Debugf("%s found in the list", domain)
		err := mess.AddValue("blocked", "1")
		rr, err := dns.NewRR(fmt.Sprintf("%s A 0.0.0.0", domain))
		if err != nil {
			return mess, err
		}
		m.Answer = append(m.Answer, rr)
		mess.SetMsg(m)
		mess.Resolved(true)
		return mess, nil
	}
	logger.Debugf("%s not found in the list", domain)
	return mess, nil

}

func (bp *BlackListPlugin) Config(c config.Config) error {
	bp.Enabled = c.BlackHole.Enabled
	bf := c.BlackHole.File
	ef := c.BlackHole.ExternalFile
	er := c.BlackHole.ExternalRepo
	bp.ctx, bp.cancel = context.WithCancel(context.Background())
	if bf != "" {
		bp.HoleFile = bf
		bp.WhiteList = c.BlackHole.Excludes
		bp.externalFile = ef
		bp.externalRepo = er
		bp.DefaultAddr = BLAddr
		bp.branch = "master"
		return nil
	}
	return errors.New("BlackholeFile is mandatory")
}

type GitLister interface {
	List(o *git.ListOptions) (rfs []*plumbing.Reference, err error)
}

type GitDownloader struct {
	externalFile string
	externalRepo string
	branch       string
	holeFile     string
	remote       GitLister
}

func (gd *GitDownloader) RemoteHEAD() (string, error) {
	logger := log.GetLogger("blacklist", "RemoteHEAD")
	logger.Debugf("Checking remote HEAD... %s", gd.externalRepo)
	refs, err := gd.remote.List(&git.ListOptions{
		PeelingOption: git.AppendPeeled,
	})
	if err != nil {
		return "", err
	}
	for _, r := range refs {
		if r.Name().String() == fmt.Sprintf("refs/heads/%s", gd.branch) {
			return r.Hash().String(), nil
		}
	}
	return "", errors.New("could not find remote HEAD")
}

// --
func (gd *GitDownloader) ReadLastHash(stateFile string) string {
	b, err := os.ReadFile(stateFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func (gd *GitDownloader) WriteLastHash(h string, stateFile string) {
	_ = os.WriteFile(stateFile, []byte(h+"\n"), 0644)
}

func (gd *GitDownloader) GithubRaw(file *os.File, owner, repo, path, ref string) (uint64, error) {
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
		owner, repo, url.PathEscape(path), ref)
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("Accept", "application/vnd.github.v3.raw")

	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "token "+tok)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return 0, fmt.Errorf("GitHub API %d: %s", resp.StatusCode, bytes.TrimSpace(slurp))
	}
	counter := &WriteCounter{
		logger:   log.GetLogger("BlacklistPlugin", "Download"),
		ReportMB: 1,
	}
	if _, err = io.Copy(file, io.TeeReader(resp.Body, counter)); err != nil {
		return counter.Total, err
	}
	return counter.Total, nil

}

//--

func (bp *BlackListPlugin) Download() error {
	logger := log.GetLogger("plugin.BlackListPlugin", "Download")
	if bp.externalFile != "" && bp.externalRepo != "" {
		head, err := remoteHEAD(bp.externalRepo, bp.branch)
		if err != nil {
			logger.Fatalf("Could not query remote: %v", err)
		}
		last := readLastHash(bp.HoleFile + ".state")
		if head == last {
			logger.Info("No changes to remote!")
			return nil
		}
		owner, repo, err := parseGitHub(bp.externalRepo)
		if err != nil {
			return err
		}
		logger.Info("Downloading remote hosts file.")
		f, err := os.OpenFile(bp.HoleFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0755)
		defer func() {
			_ = f.Close()
		}()
		if err != nil {
			logger.Error(err)
			return err
		}
		total, err := githubRaw(f, owner, repo, bp.externalFile, head)
		if err != nil {
			return err
		}
		logger.Infof("Downloaded %s from remote hosts file.", humanize.Bytes(total))
		writeLastHash(head, bp.HoleFile+".state")
	}
	return nil
}

func (bp *BlackListPlugin) Init() error {

	err := bp.Download()
	if err != nil {
		return err
	}
	readFile, err := os.Open(bp.HoleFile)
	if err != nil {
		return err
	}
	defer func(readFile *os.File) {
		err := readFile.Close()
		if err != nil {
			logger := log.GetLogger("blacklist", "Init/deferred")
			logger.Errorf("Error closing file %s: %s", bp.HoleFile, err.Error())
		}
	}(readFile)
	scanner := bufio.NewScanner(readFile)
OUTER:
	for scanner.Scan() {
		s := scanner.Text()
		if strings.HasPrefix(s, "#") {
			continue
		}

		fields := strings.Fields(s)
		if len(fields) > 1 {
			for _, each := range bp.WhiteList {
				if strings.HasSuffix(fields[1], each) {
					continue OUTER
				}
			}
			bp.Hole.Insert(fields[1], None{})
		}
	}

	cf := config.GetRunningConfig()
	mf := int64(BLMaxFuzziness * time.Second)
	if cf.BlackHole.ExternalPullPeriod != "" {
		name := "BlackListDownloader"
		t := sched.Task{
			Name: name,
			Fn: sched.FuzzyTask(name, bp.ctx, mf, func(context.Context) {
				err := bp.Download()
				if err != nil {
					panic(err)
				}
			}),
			Expr: cf.BlackHole.ExternalPullPeriod,
		}
		sched.Add(t)
	}

	return nil

}

func remoteHEAD(repoURL string, branch string) (string, error) {
	logger := log.GetLogger("blacklist", "RemoteHEAD")
	logger.Debugf("Checking remote HEAD... %s", repoURL)
	remote := git.NewRemote(memory.NewStorage(), &gitconf.RemoteConfig{
		Name: "origin",
		URLs: []string{repoURL},
	})
	refs, err := remote.List(&git.ListOptions{
		PeelingOption: git.AppendPeeled,
	})
	if err != nil {
		return "", err
	}
	for _, r := range refs {
		if r.Name().String() == fmt.Sprintf("refs/heads/%s", branch) {
			return r.Hash().String(), nil
		}
	}
	return "", errors.New("could not find remote HEAD")
}

func readLastHash(stateFile string) string {
	b, err := os.ReadFile(stateFile)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func writeLastHash(h string, stateFile string) {
	_ = os.WriteFile(stateFile, []byte(h+"\n"), 0644)
}

func githubRaw(file *os.File, owner, repo, path, ref string) (uint64, error) {
	endpoint := fmt.Sprintf("https://api.github.com/repos/%s/%s/contents/%s?ref=%s",
		owner, repo, url.PathEscape(path), ref)
	req, _ := http.NewRequest("GET", endpoint, nil)
	req.Header.Set("Accept", "application/vnd.github.v3.raw")

	if tok := os.Getenv("GITHUB_TOKEN"); tok != "" {
		req.Header.Set("Authorization", "token "+tok)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return 0, fmt.Errorf("GitHub API %d: %s", resp.StatusCode, bytes.TrimSpace(slurp))
	}
	counter := &WriteCounter{
		logger:   log.GetLogger("BlacklistPlugin", "Download"),
		ReportMB: 1,
	}
	if _, err = io.Copy(file, io.TeeReader(resp.Body, counter)); err != nil {
		return counter.Total, err
	}
	return counter.Total, nil

}

func parseGitHub(uri string) (owner, repo string, err error) {
	// Accept https://github.com/owner/repo(.git) or git@github.com:owner/repo.git
	if strings.HasPrefix(uri, "http") {
		u, err := url.Parse(uri)
		if err != nil {
			return "", "", err
		}
		parts := strings.Split(strings.TrimSuffix(u.Path, ".git"), "/")
		if len(parts) < 3 {
			return "", "", fmt.Errorf("unexpected URL form")
		}
		owner, repo = parts[1], parts[2]
	} else {
		return "", "", fmt.Errorf("unrecognised GitHub URL")
	}
	return owner, repo, nil
}
