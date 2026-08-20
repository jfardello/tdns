package packaging

import (
	"os"
	"os/exec"
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

func TestReleaseChecksumNameMatchesPackagingRecipes(t *testing.T) {
	releaseConfig := readAsset(t, "..", ".goreleaser.yaml")
	if !strings.Contains(releaseConfig, `name_template: "{{ .ProjectName }}_{{ .Version }}_checksums.txt"`) {
		t.Error("GoReleaser checksum name is not the versioned filename expected by packaging")
	}
	if !strings.Contains(releaseConfig, `version_template: "{{ incpatch .Version }}~dev.g{{ .ShortCommit }}"`) {
		t.Error("GoReleaser snapshots do not use the package-safe development version")
	}

	spec := readAsset(t, "rpm", "tdns.spec")
	if !strings.Contains(spec, "tdns_%{version}_checksums.txt") {
		t.Error("RPM recipe does not use the versioned release checksum filename")
	}

	updater := readAsset(t, "obs", "update-package.sh")
	if !strings.Contains(updater, "checksums=tdns_${version}_checksums.txt") {
		t.Error("OBS updater does not use the versioned release checksum filename")
	}
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
		"%{_mandir}/man1/tdns*.1*",
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

func TestDebianRecipePolicy(t *testing.T) {
	if got, want := readAsset(t, "debian", "tdns.service"), readAsset(t, "common", "tdns.service"); got != want {
		t.Error("Debian service unit has drifted from the shared service policy")
	}

	control := readAsset(t, "debian", "control")
	for _, expected := range []string{
		"Build-Depends: debhelper-compat (= 13), file",
		"Architecture: amd64 arm64 armhf",
		"Depends: ${misc:Depends}, systemd",
	} {
		if !strings.Contains(control, expected) {
			t.Errorf("Debian control is missing %q", expected)
		}
	}

	if got, want := strings.TrimSpace(readAsset(t, "debian", "source", "format")), "3.0 (quilt)"; got != want {
		t.Fatalf("Debian source format = %q, want %q", got, want)
	}

	rules := readAsset(t, "debian", "rules")
	for _, expected := range []string{
		"ifeq ($(DEB_HOST_ARCH),amd64)",
		"TDNS_COMPONENT := arm64",
		"TDNS_COMPONENT := armhf",
		"packaging/common",
		"man --output-dir",
		"dh_installsystemd --no-enable --no-start --no-stop-on-upgrade tdns.service",
	} {
		if !strings.Contains(rules, expected) {
			t.Errorf("Debian rules are missing %q", expected)
		}
	}
	for _, forbidden := range []string{"curl ", "wget ", "latest/download", "systemctl"} {
		if strings.Contains(rules, forbidden) {
			t.Errorf("Debian rules contain forbidden operation %q", forbidden)
		}
	}

	postinst := readAsset(t, "debian", "tdns.postinst")
	for _, expected := range []string{
		`[ -n "${2:-}" ]`,
		"deb-systemd-invoke is-active tdns.service",
		"deb-systemd-invoke try-restart tdns.service",
		"systemd-sysusers /usr/lib/sysusers.d/tdns.conf",
		"systemd-tmpfiles --create /usr/lib/tmpfiles.d/tdns.conf",
		"chown root:tdns /etc/tdns",
		"chmod 0750 /etc/tdns",
	} {
		if !strings.Contains(postinst, expected) {
			t.Errorf("Debian postinst is missing %q", expected)
		}
	}
	for _, forbidden := range []string{"tdns config", "systemctl", "rm -"} {
		if strings.Contains(postinst, forbidden) {
			t.Errorf("Debian postinst contains forbidden operation %q", forbidden)
		}
	}

	prerm := readAsset(t, "debian", "tdns.prerm")
	if !strings.Contains(prerm, `[ "$1" = "remove" ]`) || !strings.Contains(prerm, "deb-systemd-invoke stop tdns.service") {
		t.Error("Debian prerm does not stop an active service specifically on removal")
	}
	if strings.Contains(prerm, "upgrade") || strings.Contains(prerm, "systemctl") {
		t.Error("Debian prerm must not stop TDNS during upgrade or invoke systemctl directly")
	}

	for _, executable := range []string{"rules", "tdns.postinst", "tdns.prerm"} {
		info, err := os.Stat(filepath.Join("debian", executable))
		if err != nil {
			t.Fatalf("stat Debian %s: %v", executable, err)
		}
		if info.Mode().Perm() != 0o755 {
			t.Errorf("Debian %s mode = %04o, want 0755", executable, info.Mode().Perm())
		}
	}
}

func TestDebianPostinstServiceLifecycle(t *testing.T) {
	binDir := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "maintainer.log")
	helper := `#!/bin/sh
name=${0##*/}
printf '%s %s\n' "$name" "$*" >> "$TDNS_MAINT_LOG"
if [ "$name" = "deb-systemd-invoke" ] && [ "$1" = "is-active" ] && [ "${TDNS_SERVICE_ACTIVE:-0}" != "1" ]; then
	exit 3
fi
`
	for _, name := range []string{"systemd-sysusers", "systemd-tmpfiles", "chown", "chmod", "deb-systemd-invoke"} {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(helper), 0o755); err != nil {
			t.Fatalf("write fake %s: %v", name, err)
		}
	}

	runPostinst := func(previousVersion string, active bool) string {
		t.Helper()
		if err := os.WriteFile(logPath, nil, 0o600); err != nil {
			t.Fatalf("reset maintainer log: %v", err)
		}
		args := []string{"debian/tdns.postinst", "configure"}
		if previousVersion != "" {
			args = append(args, previousVersion)
		}
		command := exec.Command("/bin/sh", args...)
		activeValue := "0"
		if active {
			activeValue = "1"
		}
		command.Env = append(os.Environ(),
			"PATH="+binDir,
			"TDNS_MAINT_LOG="+logPath,
			"TDNS_SERVICE_ACTIVE="+activeValue,
		)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("run postinst: %v\n%s", err, output)
		}
		return readAsset(t, logPath)
	}

	fresh := runPostinst("", false)
	if strings.Contains(fresh, "deb-systemd-invoke") {
		t.Errorf("fresh install attempted a service operation:\n%s", fresh)
	}

	inactiveUpgrade := runPostinst("0.1.8-1", false)
	if !strings.Contains(inactiveUpgrade, "deb-systemd-invoke is-active tdns.service") || strings.Contains(inactiveUpgrade, "try-restart") {
		t.Errorf("inactive upgrade used the wrong service lifecycle:\n%s", inactiveUpgrade)
	}

	activeUpgrade := runPostinst("0.1.8-1", true)
	if !strings.Contains(activeUpgrade, "deb-systemd-invoke try-restart tdns.service") {
		t.Errorf("active upgrade did not restart TDNS:\n%s", activeUpgrade)
	}
}
