package unit_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestTheManifestFrameworkFloorMatchesGoMod(t *testing.T) {
	root := packageRoot(t)
	goMod := readReleaseFile(t, root, "go.mod")
	manifest := readReleaseFile(t, root, "arandu.mod.toml")

	required := captureReleaseValue(t, goMod,
		`(?m)^\s*github\.com/arandu-io/framework v([0-9]+\.[0-9]+)\.[0-9]+\s*$`,
		"Framework version in go.mod")
	declared := captureReleaseValue(t, manifest,
		`(?m)^framework = ">= ([0-9]+\.[0-9]+)"$`,
		"Framework floor in arandu.mod.toml")

	if declared != required {
		t.Fatalf("manifest Framework floor = %s, want %s from go.mod", declared, required)
	}
}

func TestTheReleaseSkillUsesTheManifestFrameworkFloor(t *testing.T) {
	root := packageRoot(t)
	manifest := readReleaseFile(t, root, "arandu.mod.toml")
	skill := readReleaseFile(t, root, ".agents/skills/skeleton-release/SKILL.md")
	declared := captureReleaseValue(t, manifest,
		`(?m)^framework = ">= ([0-9]+\.[0-9]+)"$`,
		"Framework floor in arandu.mod.toml")

	want := `framework = ">= ` + declared + `"`
	if !strings.Contains(skill, want) {
		t.Fatalf("release skill does not teach manifest floor %q", want)
	}
}

func TestVersion020NamesEveryModelFirstIncompatibility(t *testing.T) {
	root := packageRoot(t)
	upgrade := readReleaseFile(t, root, "UPGRADE.md")
	changelog := readReleaseFile(t, root, "CHANGELOG.md")

	if got := strings.Count(changelog, "## [0.2.0] - "); got != 1 {
		t.Fatalf("v0.2.0 changelog headings = %d, want exactly one pre-versioned entry", got)
	}
	if !strings.Contains(upgrade, "## v0.2.0") {
		t.Fatal("UPGRADE.md has no v0.2.0 entry")
	}

	incompatibilities := []string{
		"`(*SkeletonService).Create`",
		"`(*SkeletonService).Find`",
		"`(*SkeletonService).List`",
		"`(*SkeletonRepository).Create`",
		"`(*SkeletonRepository).Delete`",
		"`(*SkeletonRepository).Find`",
		"`(*SkeletonRepository).List`",
		"`(*SkeletonRepository).Update`",
		"`NewSkeletonRepository`",
		"`NewSkeletonService`",
		"`SkeletonRepository`",
		"`Skeleton`: old is comparable; new is not",
	}
	for _, incompatibility := range incompatibilities {
		if !strings.Contains(upgrade, incompatibility) {
			t.Errorf("UPGRADE.md does not name %s", incompatibility)
		}
		if !strings.Contains(changelog, incompatibility) {
			t.Errorf("v0.2.0 notes do not name %s", incompatibility)
		}
	}
}

// TestVersion040NamesEveryPublishingIncompatibility holds the same promise for
// the release that moved publishing to the framework contract.
//
// The CI job that runs apidiff makes it as well, and only there: it needs the
// release tag and the git history, so a working tree that dropped a symbol
// without saying so is green locally until a pull request opens. This is the
// half that fails where the change is written.
func TestVersion040NamesEveryPublishingIncompatibility(t *testing.T) {
	root := packageRoot(t)
	upgrade := readReleaseFile(t, root, "UPGRADE.md")
	changelog := readReleaseFile(t, root, "CHANGELOG.md")

	if got := strings.Count(changelog, "## [0.4.0] - "); got != 1 {
		t.Fatalf("v0.4.0 changelog headings = %d, want exactly one pre-versioned entry", got)
	}
	if !strings.Contains(upgrade, "## v0.4.0") {
		t.Fatal("UPGRADE.md has no v0.4.0 entry")
	}

	incompatibilities := []string{
		"`Publishable`",
		"`Publishes`",
		"`PublishCommand`",
		"`foundation.Publishable`",
		"aru vendor:publish",
	}
	for _, incompatibility := range incompatibilities {
		if !strings.Contains(upgrade, incompatibility) {
			t.Errorf("UPGRADE.md does not name %s", incompatibility)
		}
		if !strings.Contains(changelog, incompatibility) {
			t.Errorf("v0.4.0 notes do not name %s", incompatibility)
		}
	}
}

func TestCIGuardsIncompatibleAPIChanges(t *testing.T) {
	ci := readReleaseFile(t, packageRoot(t), ".github/workflows/ci.yml")
	required := []string{
		"fetch-depth: 0",
		"name: api diff against the last release",
		`modpath=$(GOWORK=off go list -m -f '{{.Path}}')`,
		`git show "${tag}:go.mod"`,
		"git worktree add",
		"golang.org/x/exp/cmd/apidiff@latest -m -w",
		"-m -incompatible",
		`git diff --quiet "$latest" -- UPGRADE.md`,
		`added=$(git diff "$latest" -- UPGRADE.md`,
	}
	for _, want := range required {
		if !strings.Contains(ci, want) {
			t.Errorf("CI does not contain the API compatibility gate %q", want)
		}
	}
}

func TestTheReleasePublishesThePreVersionedChangelogEntryOnce(t *testing.T) {
	root := packageRoot(t)
	release := readReleaseFile(t, root, ".github/workflows/release.yml")
	changelog := readReleaseFile(t, root, "CHANGELOG.md")

	if got := strings.Count(changelog, "## [0.2.0] - "); got != 1 {
		t.Fatalf("v0.2.0 changelog headings = %d, want one", got)
	}
	required := []string{
		`tags: ["v*"]`,
		"contents: write",
		`version="${TAG#v}"`,
		`heading="## [$version] - "`,
		`gh release create "$TAG"`,
		"--verify-tag",
		`--notes-file "$notes"`,
	}
	for _, want := range required {
		if !strings.Contains(release, want) {
			t.Errorf("release workflow does not contain %q", want)
		}
	}
	if got := strings.Count(release, `gh release create "$TAG"`); got != 1 {
		t.Errorf("release creation commands = %d, want exactly one", got)
	}
	for _, forbidden := range []string{
		"Write it into the changelog",
		"git commit",
		"git push",
		"git tag",
		"gh release delete",
		"CHANGELOG.next",
	} {
		if strings.Contains(release, forbidden) {
			t.Errorf("release workflow mutates the pre-versioned changelog through %q", forbidden)
		}
	}
}

func readReleaseFile(t *testing.T, root, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(raw)
}

func captureReleaseValue(t *testing.T, body, pattern, label string) string {
	t.Helper()
	match := regexp.MustCompile(pattern).FindStringSubmatch(body)
	if len(match) != 2 {
		t.Fatalf("%s is missing", label)
	}
	return match[1]
}
