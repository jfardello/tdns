package packaging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readAsset(t *testing.T, path ...string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(path...))
	if err != nil {
		t.Fatalf("read %s: %v", filepath.Join(path...), err)
	}
	return string(contents)
}

func TestAccountAndDirectoryAssets(t *testing.T) {
	if got, want := strings.TrimSpace(readAsset(t, "common", "tdns.sysusers")),
		"# Type Name ID GECOS Home directory Shell\nu tdns - \"TDNS DNS resolver\" /var/lib/tdns -"; got != want {
		t.Fatalf("unexpected sysusers policy:\n%s", got)
	}
	if got, want := strings.TrimSpace(readAsset(t, "common", "tdns.tmpfiles")),
		"# Type Path Mode User Group Age Argument\nd /var/lib/tdns 0750 tdns tdns - -"; got != want {
		t.Fatalf("unexpected tmpfiles policy:\n%s", got)
	}
}

func TestRPMRecipePolicy(t *testing.T) {
	spec := readAsset(t, "rpm", "tdns.spec")
	for _, expected := range []string{
		"%global debug_package %{nil}",
		"ExclusiveArch:  x86_64 aarch64",
		"releases/download/v%{version}/tdns_Linux_x86_64.tar.gz",
		"releases/download/v%{version}/tdns_Linux_arm64.tar.gz",
		"sha256sum --check --status",
		"file -b tdns | grep -Fq \"%{tdns_machine}\"",
		"./tdns --version",
		"./tdns man --output-dir %{buildroot}%{_mandir}/man1",
		"%service_add_post tdns.service",
		"%systemd_post tdns.service",
		"%service_del_postun tdns.service",
		"%systemd_postun_with_restart tdns.service",
		"%attr(0750,root,tdns) %{_sysconfdir}/tdns",
		"%attr(0750,tdns,tdns) %{_sharedstatedir}/tdns",
		"%{_mandir}/man1/tdns*.1%{?ext_man}",
	} {
		if !strings.Contains(spec, expected) {
			t.Errorf("RPM recipe is missing %q", expected)
		}
	}

	for _, forbidden := range []string{
		"systemctl enable",
		"systemctl start",
		"setcap",
		"useradd",
		"groupadd",
		"latest/download",
	} {
		if strings.Contains(spec, forbidden) {
			t.Errorf("RPM recipe contains forbidden operation %q", forbidden)
		}
	}

	rpmlintrc := readAsset(t, "rpm", "tdns-rpmlintrc")
	if strings.Contains(rpmlintrc, "no-manual-page-for-binary") {
		t.Error("RPM lint policy still suppresses the missing manual page diagnostic")
	}
}

func TestBootstrapDocumentationDoesNotHidePackageSideEffects(t *testing.T) {
	doc := readAsset(t, "common", "README.packaging")
	for _, expected := range []string{
		"--systemd-unit=false",
		"chown root:tdns /etc/tdns/tdns.yaml",
		"chmod 0640 /etc/tdns/tdns.yaml",
		"systemctl enable --now tdns.service",
		"does not enable or start the service",
		"does not delete operator-created files",
	} {
		if !strings.Contains(doc, expected) {
			t.Errorf("package documentation is missing %q", expected)
		}
	}
}
