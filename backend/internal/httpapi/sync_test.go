package httpapi

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/stacktraceo/league-companion/backend/internal/domain"
	"github.com/stacktraceo/league-companion/backend/internal/syncer"
)

const syncPath = "/api/v1/summoners/" + testPUUID + "/sync"

func syncDeps(lastSyncedAt *time.Time) Deps {
	deps := testDeps()

	summoner := testSummoner()
	summoner.LastSyncedAt = lastSyncedAt
	deps.Summoners = &fakeSummoners{items: map[string]domain.Summoner{testPUUID: summoner}}

	return deps
}

func TestForceSyncQueuesNeverSyncedSummoner(t *testing.T) {
	deps := syncDeps(nil)
	queue := deps.Queue.(*fakeQueue)

	rec := call(t, deps, http.MethodPost, syncPath, "")
	require.Equal(t, http.StatusAccepted, rec.Code, "тело: %s", rec.Body.String())

	body := decodeBody[SyncAcceptedResponse](t, rec)
	assert.Equal(t, "queued", body.Status)
	assert.Equal(t, testPUUID, body.PUUID)
	assert.Nil(t, body.LastSyncedAt)

	assert.Equal(t, []string{testPUUID}, queue.enqueued)
}

func TestForceSyncQueuesAfterCooldown(t *testing.T) {
	synced := testNow.Add(-manualSyncCooldown - time.Second)
	deps := syncDeps(&synced)
	queue := deps.Queue.(*fakeQueue)

	rec := call(t, deps, http.MethodPost, syncPath, "")
	require.Equal(t, http.StatusAccepted, rec.Code, "тело: %s", rec.Body.String())

	// Отдаём отметку до прогона: по её сдвигу клиент поймёт, что синхронизация прошла.
	body := decodeBody[SyncAcceptedResponse](t, rec)
	require.NotNil(t, body.LastSyncedAt)
	assert.Equal(t, synced.UTC(), body.LastSyncedAt.UTC())

	assert.Equal(t, []string{testPUUID}, queue.enqueued)
}

func TestForceSyncRejectsWithinCooldown(t *testing.T) {
	synced := testNow.Add(-30 * time.Second)
	deps := syncDeps(&synced)
	queue := deps.Queue.(*fakeQueue)

	rec := call(t, deps, http.MethodPost, syncPath, "")
	requireErrorCode(t, rec, http.StatusTooManyRequests, "sync_too_soon")

	// Осталось 90 секунд из двух минут.
	assert.Equal(t, "90", rec.Header().Get("Retry-After"))
	assert.Empty(t, queue.enqueued, "в очередь ставить нельзя")
}

func TestForceSyncRetryAfterRoundsUp(t *testing.T) {
	synced := testNow.Add(-manualSyncCooldown + 200*time.Millisecond)
	rec := call(t, syncDeps(&synced), http.MethodPost, syncPath, "")

	requireErrorCode(t, rec, http.StatusTooManyRequests, "sync_too_soon")

	retryAfter, err := strconv.Atoi(rec.Header().Get("Retry-After"))
	require.NoError(t, err)
	assert.Equal(t, 1, retryAfter)
}

func TestForceSyncAcceptsExactlyAtCooldownBoundary(t *testing.T) {
	synced := testNow.Add(-manualSyncCooldown)

	rec := call(t, syncDeps(&synced), http.MethodPost, syncPath, "")
	assert.Equal(t, http.StatusAccepted, rec.Code, "тело: %s", rec.Body.String())
}

func TestForceSyncReportsFullQueue(t *testing.T) {
	deps := syncDeps(nil)
	deps.Queue = &fakeQueue{reject: true}

	rec := call(t, deps, http.MethodPost, syncPath, "")
	requireErrorCode(t, rec, http.StatusServiceUnavailable, "sync_queue_full")
}

func TestForceSyncReturns404ForUnknownSummoner(t *testing.T) {
	rec := call(t, testDeps(), http.MethodPost, "/api/v1/summoners/нет-такого/sync", "")
	requireErrorCode(t, rec, http.StatusNotFound, "summoner_not_found")
}

func TestForceSyncReturns500OnStorageFailure(t *testing.T) {
	deps := testDeps()
	deps.Summoners = &fakeSummoners{err: errDatabaseDown}

	rec := call(t, deps, http.MethodPost, syncPath, "")
	requireErrorCode(t, rec, http.StatusInternalServerError, "internal_error")
}

func TestForceSyncUsesDefaultMatchCount(t *testing.T) {
	deps := syncDeps(nil)
	queue := &countingQueue{}
	deps.Queue = queue

	require.Equal(t, http.StatusAccepted, call(t, deps, http.MethodPost, syncPath, "").Code)
	assert.Equal(t, syncer.DefaultMatchCount, queue.count)
}
