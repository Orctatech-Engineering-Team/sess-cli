package main

import (
	"time"

	"github.com/Orctatech-Engineering-Team/Sess/internal/sess"
	"github.com/earthboundkid/versioninfo/v2"
)

func main() {
	sess.SetVersionInfo(versioninfo.Version, versioninfo.Revision, versioninfo.LastCommit.Format(time.RFC3339))
	sess.Execute()
}
