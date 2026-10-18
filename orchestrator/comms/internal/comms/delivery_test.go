package comms

import (
	"bytes"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func deliveryFixture(t *testing.T, s *Store) (Task, Participant, Participant, ClaimDeliveryInput) {
	t.Helper()
	task := testTask(t, s, "delivery-project")
	owner := testParticipant(t, s, task, task.Owner)
	worker := testParticipant(t, s, task, "coder")
	testAssignment(t, s, task, owner, worker)
	inbox, err := s.Inbox(t.Context(), InboxInput{TaskID: task.ID, Recipient: worker.RoutingIdentity})
	require.NoError(t, err)
	return task, owner, worker, ClaimDeliveryInput{TaskID: task.ID, ParticipantID: worker.ID, MessageID: inbox[0].ID}
}
func TestDeliveryClaimsAreExclusiveAndDoNotAcceptWork(t *testing.T) {
	s := testStore(t)
	task, _, worker, input := deliveryFixture(t, s)
	var wg sync.WaitGroup
	claims := make(chan DeliveryClaim, 8)
	failures := make(chan error, 8)
	for range 8 {
		wg.Go(func() { c, e := s.ClaimDelivery(t.Context(), input); claims <- c; failures <- e })
	}
	wg.Wait()
	close(claims)
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
	var claim *Delivery
	count := 0
	for c := range claims {
		if c.Delivery != nil {
			count++
			claim = c.Delivery
		}
	}
	require.Equal(t, 1, count)
	finish := FinishDeliveryInput{ClaimDeliveryInput: input, LeaseToken: claim.LeaseToken, State: "submitted", Detail: "input submitted"}
	d, err := s.FinishDelivery(t.Context(), finish)
	require.NoError(t, err)
	assert.Equal(t, "submitted", d.State)
	_, err = s.FinishDelivery(t.Context(), finish)
	require.NoError(t, err, "lost finish response is safely replayed")
	c, err := s.ClaimDelivery(t.Context(), input)
	require.NoError(t, err)
	assert.Nil(t, c.Delivery)
	inbox, err := s.Inbox(t.Context(), InboxInput{TaskID: task.ID, Recipient: worker.RoutingIdentity})
	require.NoError(t, err)
	assert.Len(t, inbox, 1, "delivery is not reading")
	assignments, err := s.Assignments(t.Context(), ListAssignmentsInput{TaskID: task.ID})
	require.NoError(t, err)
	assert.Equal(t, "offered", assignments[0].State)
	report, err := s.Report(t.Context(), task.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, report.DeliveryCounts["submitted"])
	assert.Empty(t, report.Deliveries[0].LeaseToken)
	var html bytes.Buffer
	require.NoError(t, RenderReport(&html, report))
	assert.Contains(t, html.String(), "Inbox delivery")
}
func TestDeliveryCrashRecoveryRequiresExplicitRetry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	s, err := Open(path)
	require.NoError(t, err)
	task, owner, worker, input := deliveryFixture(t, s)
	claim, err := s.ClaimDelivery(t.Context(), input)
	require.NoError(t, err)
	require.NotNil(t, claim.Delivery)
	require.NoError(t, s.Close())
	s, err = Open(path)
	require.NoError(t, err)
	defer s.Close()
	later := time.Now().Add(time.Minute)
	next, err := s.claimDeliveryAt(t.Context(), input, later)
	require.NoError(t, err)
	assert.Nil(t, next.Delivery)
	records, err := s.Deliveries(t.Context(), TaskInput{TaskID: task.ID})
	require.NoError(t, err)
	assert.Equal(t, "uncertain", records[0].State)
	_, err = s.FinishDelivery(t.Context(), FinishDeliveryInput{ClaimDeliveryInput: input, LeaseToken: claim.Delivery.LeaseToken, State: "submitted", Detail: "late"})
	require.ErrorContains(t, err, "stale")
	retry := RetryDeliveryInput{ClaimDeliveryInput: input, Actor: worker.ID, RequestKey: "retry"}
	_, err = s.RetryDelivery(t.Context(), retry)
	require.ErrorContains(t, err, "owner")
	retry.Actor = owner.ID
	_, err = s.RetryDelivery(t.Context(), retry)
	require.NoError(t, err)
	_, err = s.RetryDelivery(t.Context(), retry)
	require.NoError(t, err)
	next, err = s.ClaimDelivery(t.Context(), input)
	require.NoError(t, err)
	require.NotNil(t, next.Delivery)
	notices, err := s.Inbox(t.Context(), InboxInput{TaskID: task.ID, Recipient: owner.RoutingIdentity})
	require.NoError(t, err)
	assert.Len(t, notices, 1)
}
func TestDeliveryBackoffAlertsAndBoundaries(t *testing.T) {
	s := testStore(t)
	task, owner, worker, input := deliveryFixture(t, s)
	for i := 1; i <= 5; i++ {
		claim, err := s.ClaimDelivery(t.Context(), input)
		require.NoError(t, err)
		require.NotNil(t, claim.Delivery)
		finish := FinishDeliveryInput{ClaimDeliveryInput: input, LeaseToken: claim.Delivery.LeaseToken, State: "retry", Detail: "target unavailable"}
		record, err := s.FinishDelivery(t.Context(), finish)
		require.NoError(t, err)
		_, err = s.FinishDelivery(t.Context(), finish)
		require.NoError(t, err)
		if i < 5 {
			assert.Equal(t, "retry", record.State)
			claim, err = s.ClaimDelivery(t.Context(), input)
			require.NoError(t, err)
			assert.Nil(t, claim.Delivery)
			_, err = s.db.Exec("UPDATE deliveries SET retry_at=0")
			require.NoError(t, err)
		} else {
			assert.Equal(t, "failed", record.State)
		}
	}
	notices, err := s.Inbox(t.Context(), InboxInput{TaskID: task.ID, Recipient: owner.RoutingIdentity})
	require.NoError(t, err)
	require.Len(t, notices, 1, "retry failures must not flood the owner")
	ownerClaim, err := s.ClaimDelivery(t.Context(), ClaimDeliveryInput{TaskID: task.ID, MessageID: notices[0].ID, ParticipantID: owner.ID})
	require.NoError(t, err)
	_, err = s.FinishDelivery(t.Context(), FinishDeliveryInput{ClaimDeliveryInput: ClaimDeliveryInput{TaskID: task.ID, MessageID: notices[0].ID, ParticipantID: owner.ID}, LeaseToken: ownerClaim.Delivery.LeaseToken, State: "failed", Detail: "owner also unavailable"})
	require.NoError(t, err)
	notices, err = s.Inbox(t.Context(), InboxInput{TaskID: task.ID, Recipient: owner.RoutingIdentity})
	require.NoError(t, err)
	assert.Len(t, notices, 1, "failure alerts cannot recursively generate alerts")
	input.ParticipantID = owner.ID
	_, err = s.ClaimDelivery(t.Context(), input)
	require.ErrorContains(t, err, "different participant")
	input.ParticipantID = worker.ID
	input.TaskID = testTask(t, s, "other").ID
	_, err = s.ClaimDelivery(t.Context(), input)
	require.Error(t, err)
}
func TestDeliveryDoesNotSendAnOldAssignmentToReplacement(t *testing.T) {
	s := testStore(t)
	task, _, worker, input := deliveryFixture(t, s)
	replacement, err := s.RegisterParticipant(t.Context(), RegisterParticipantInput{TaskID: task.ID, InstanceID: "new-coder", Role: worker.Role, RoutingIdentity: worker.RoutingIdentity, ReplacesParticipantID: worker.ID, RequestKey: "replacement"})
	require.NoError(t, err)
	_, err = s.ClaimDelivery(t.Context(), input)
	require.ErrorContains(t, err, "retired")
	input.ParticipantID = replacement.ID
	c, err := s.ClaimDelivery(t.Context(), input)
	require.NoError(t, err)
	assert.Nil(t, c.Delivery)
}
func TestSchemaTwoDeliveryUpgrade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	s, err := Open(path)
	require.NoError(t, err)
	task := testTask(t, s, "old")
	_, err = s.db.Exec("DROP TABLE deliveries; PRAGMA user_version=2")
	require.NoError(t, err)
	require.NoError(t, s.Close())
	s, err = Open(path)
	require.NoError(t, err)
	defer s.Close()
	report, err := s.Report(t.Context(), task.ID)
	require.NoError(t, err)
	assert.Equal(t, task.ID, report.Task.ID)
	assert.Empty(t, report.Deliveries)
}
