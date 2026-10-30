package engine

import "runtime"

// Pinned engine artifact versions (M2c). Bump with checksums together.
const (
	pinnedColima = "v0.8.1"
	pinnedLima   = "v1.0.6"
	pinnedDocker = "27.5.1"
)

type artifactKind int

const (
	artifactRaw artifactKind = iota
	artifactTarGz
)

type pinnedArtifact struct {
	Name   string // installed binary name in bin/
	URL    string
	SHA256 string
	Kind   artifactKind
	Member string // path inside archive for TarGz (e.g. docker/docker)
	// ExtractRoot, if set, extracts the whole tar.gz under root/ExtractRoot (for Lima share/).
	ExtractRoot string
	Required    bool
	// ExpectedBytes guides UI progress when Content-Length is missing (approx).
	ExpectedBytes int64
}

func pinnedArtifacts() []pinnedArtifact {
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/arm64":
		return []pinnedArtifact{
			{
				Name: "colima", Required: true, Kind: artifactRaw, ExpectedBytes: 8_746_960,
				URL:    "https://github.com/abiosoft/colima/releases/download/" + pinnedColima + "/colima-Darwin-arm64",
				SHA256: "c266fcb272b39221ef6152d2093bb02a1ebadc26042233ad359e1ae52d5d5922",
			},
			{
				Name: "limactl", Required: true, Kind: artifactTarGz, ExtractRoot: "lima", ExpectedBytes: 63_288_670,
				URL:    "https://github.com/lima-vm/lima/releases/download/" + pinnedLima + "/lima-1.0.6-Darwin-arm64.tar.gz",
				SHA256: "4c65d63eb5f3a97a781ccea4944aa936def4d68e34e95c3f1e010791f318236c",
			},
			{
				Name: "docker", Required: true, Kind: artifactTarGz, Member: "docker/docker", ExpectedBytes: 55_000_000,
				URL:    "https://download.docker.com/mac/static/stable/aarch64/docker-" + pinnedDocker + ".tgz",
				SHA256: "b0366bf473b9b97b0c5556c4c2fd6d92ea63e4848af2f869c93a98d2e2e1d3fa",
			},
		}
	case "darwin/amd64":
		return []pinnedArtifact{
			{
				Name: "colima", Required: true, Kind: artifactRaw, ExpectedBytes: 9_000_000,
				URL:    "https://github.com/abiosoft/colima/releases/download/" + pinnedColima + "/colima-Darwin-x86_64",
				SHA256: "791330c62c60389f70e5e1c33a56c35502a9e36e544a418daea0273e539acbf4",
			},
			{
				Name: "limactl", Required: true, Kind: artifactTarGz, ExtractRoot: "lima", ExpectedBytes: 65_000_000,
				URL:    "https://github.com/lima-vm/lima/releases/download/" + pinnedLima + "/lima-1.0.6-Darwin-x86_64.tar.gz",
				SHA256: "8b3c12c6ab0c38659cf7f0c94f68bc6533d45a2adeceaac616d988b2ecb2f1b0",
			},
			{
				Name: "docker", Required: true, Kind: artifactTarGz, Member: "docker/docker", ExpectedBytes: 60_000_000,
				URL:    "https://download.docker.com/mac/static/stable/x86_64/docker-" + pinnedDocker + ".tgz",
				SHA256: "425560a99d1231433ae4b7149a4d1235461cb28783af32481741a367bd1a9f6a",
			},
		}
	default:
		return nil
	}
}
