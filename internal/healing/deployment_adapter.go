package healing

import (
	"context"
	"fmt"
)

type DeploymentAdapter struct{}

func (DeploymentAdapter) Kind() string {
	return "Deployment"
}

func (DeploymentAdapter) Supports(kind string) bool {
	return kind == "Deployment"
}

func (DeploymentAdapter) ListRevisions(_ context.Context, _, _ string) ([]RevisionRecord, error) {
	return nil, nil
}

func (DeploymentAdapter) RollbackToRevision(_ context.Context, _, _, revision string) error {
	if revision == "" {
		return fmt.Errorf("revision is required")
	}
	return nil
}
