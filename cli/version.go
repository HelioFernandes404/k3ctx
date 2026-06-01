package cli

import (
	"encoding/json"
	"fmt"

	buildversion "github.com/systemframe/k3ctx/internal/version"
)

func versionJSON() string {
	b, err := json.Marshal(buildversion.Current())
	if err != nil {
		return fmt.Sprintf("{\"version\":%q,\"commit\":%q,\"date\":%q}\n", buildversion.Version, buildversion.Commit, buildversion.Date)
	}
	return string(b) + "\n"
}
