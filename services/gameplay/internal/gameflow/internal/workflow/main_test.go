package workflow

import (
	"github.com/spazzle-io/spazzle-api/services/gameplay/internal/gameflow/internal/activities"
	"github.com/stretchr/testify/suite"
	"go.temporal.io/sdk/testsuite"
)

type WorkflowTestSuite struct {
	suite.Suite
	testsuite.WorkflowTestSuite
	env *testsuite.TestWorkflowEnvironment

	activities *activities.Activities
}

func (s *WorkflowTestSuite) SetupTest() {
	s.env = s.NewTestWorkflowEnvironment()
	s.env.RegisterWorkflow(GameWorkflow)

	s.activities = &activities.Activities{}
	s.env.RegisterActivity(s.activities)
}

func (s *WorkflowTestSuite) AfterTest(_, _ string) {
	s.env.AssertExpectations(s.T())
}
