// Testing main Logic
package clock

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCalculateOffset(t *testing.T) {
	T1 := int64(1000)
	T2 := int64(2000)
	T3 := int64(3000)

	offset := CalculateOffset(T1, T2, T3)

	expected := int64(2000 - ((1000 + 3000) / 2)) // = 0

	if offset != expected {
		t.Errorf("Expected %d but got %d", expected, offset)
	}
}

// Test server response
func TestTimeHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/time", nil)
	w := httptest.NewRecorder()

	TimeHandler(w, req)

	res := w.Result()

	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200 but got %d", res.StatusCode)
	}
}

// Test client request
func TestRequestServerTime(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(TimeHandler))
	defer mockServer.Close()

	T1, T2, T3, err := RequestServerTime(mockServer.URL)

	if err != nil {
		t.Errorf("Request failed: %v", err)
	}

	if T2 == 0 {
		t.Errorf("Expected server time but got 0")
	}

	if T3 < T1 {
		t.Errorf("Invalid timing: T3 < T1")
	}
}

// Test full flow
func TestGetSyncedTime(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(TimeHandler))
	defer mockServer.Close()

	syncedTime, err := GetSyncedTime(mockServer.URL)

	if err != nil {
		t.Errorf("Error getting synced time: %v", err)
	}

	if syncedTime == 0 {
		t.Errorf("Expected valid synced time but got 0")
	}
}
