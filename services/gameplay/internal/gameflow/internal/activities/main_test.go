package activities

import (
	"testing"

	mockworker "github.com/spazzle-io/spazzle-api/services/gameplay/internal/worker/mock"

	mockdb "github.com/spazzle-io/spazzle-api/services/gameplay/internal/db/mock"
	mockeventbus "github.com/spazzle-io/spazzle-api/services/gameplay/internal/eventbus/mock"
	mockwordstore "github.com/spazzle-io/spazzle-api/services/gameplay/internal/wordstore/mock"
	"go.uber.org/mock/gomock"
)

type ActivityTestDeps struct {
	Ctrl            *gomock.Controller
	Store           *mockdb.MockStore
	Bus             *mockeventbus.MockEventBus
	Session         *mockeventbus.MockSession
	WordStore       *mockwordstore.MockStore
	TaskDistributor *mockworker.MockTaskDistributor
	Activities      Activities
}

func setupActivities(t *testing.T) *ActivityTestDeps {
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	store := mockdb.NewMockStore(ctrl)
	bus := mockeventbus.NewMockEventBus(ctrl)
	session := mockeventbus.NewMockSession(ctrl)
	wordStore := mockwordstore.NewMockStore(ctrl)
	taskDistributor := mockworker.NewMockTaskDistributor(ctrl)

	a := Activities{
		Store:           store,
		Bus:             bus,
		WordStore:       wordStore,
		TaskDistributor: taskDistributor,
	}

	return &ActivityTestDeps{
		Ctrl:            ctrl,
		Store:           store,
		Bus:             bus,
		Session:         session,
		WordStore:       wordStore,
		TaskDistributor: taskDistributor,
		Activities:      a,
	}
}
