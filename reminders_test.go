package reminders

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/itsabot/abot/core"
	"github.com/itsabot/abot/core/log"
	"github.com/itsabot/abot/shared/datatypes"
	"github.com/julienschmidt/httprouter"
)

var r *httprouter.Router

func TestMain(m *testing.M) {
	var err error
	r, err = core.NewServer()
	if err != nil {
		log.Fatal("failed to start Abot server.", err)
	}
	exitVal := m.Run()
	q := `DELETE FROM scheduledevents`
	_, err = p.DB.Exec(q)
	if err != nil {
		log.Info("failed to delete scheduledevents.", err)
	}
	q = `DELETE FROM messages`
	_, err = p.DB.Exec(q)
	if err != nil {
		log.Info("failed to delete messages.", err)
	}
	os.Exit(exitVal)
}

func TestKWSetReminder(t *testing.T) {
	// Map test sentences with the important content that must be contained
	// in the reply.
	tests := map[string]string{
		"Remind me to buy groceries at 2pm":    "2:00PM",
		"Remind me to buy groceries next week": "buy groceries",
		"Remind me to buy groceries tomorrow":  "buy groceries",
		"Remind me":                            "",
	}
	for test, expected := range tests {
		in := core.NewMsg(&dt.User{ID: 1}, test)
		res := kwSetReminder(in)
		if !strings.Contains(res, expected) {
			t.Fatalf("expected %q, got %q\n", expected, res)
		}
	}
}

func TestStateMachine(t *testing.T) {
	testGroup := []map[string]string{
		map[string]string{
			"Remind me":        "What would you like me to remind you to do?",
			"to buy groceries": "When should I remind you?",
			"11PM":             "11:00PM",
		},
	}
	for _, tests := range testGroup {
		for test, expected := range tests {
			data := struct {
				FlexIDType int
				FlexID     string
				CMD        string
			}{
				FlexIDType: 3,
				FlexID:     "5555",
				CMD:        test,
			}
			byt, err := json.Marshal(data)
			if err != nil {
				t.Fatal("failed to marshal req.", err)
			}
			c, b := request("POST", os.Getenv("ABOT_URL")+"/", byt)
			if c != http.StatusOK {
				t.Fatal("expected", http.StatusOK, "got", c, b)
			}
			if !strings.Contains(b, expected) {
				t.Fatalf("expected %q, got %q for %q\n", expected, b, test)
			}
		}
	}
}

func TestPlugin(t *testing.T) {
	tests := map[string]string{
		"Remind me to buy groceries at 2pm":    "2:00PM",
		"Remind me to buy groceries next week": "buy groceries",
		"Remind me next week":                  "What would you like me to remind you to do?",
		"Remind me to buy groceries":           "When should I remind you?",
	}
	for test, expected := range tests {
		data := struct {
			FlexIDType int
			FlexID     string
			CMD        string
		}{
			FlexIDType: 3,
			FlexID:     "0",
			CMD:        test,
		}
		byt, err := json.Marshal(data)
		if err != nil {
			t.Fatal("failed to marshal req.", err)
		}
		c, b := request("POST", os.Getenv("ABOT_URL")+"/", byt)
		if c != http.StatusOK {
			t.Fatal("expected", http.StatusOK, "got", c, b)
		}
		if !strings.Contains(b, expected) {
			t.Fatalf("expected %q, got %q\n", expected, b)
		}
	}
}

func request(method, path string, data []byte) (int, string) {
	req, err := http.NewRequest(method, path, bytes.NewBuffer(data))
	if err != nil {
		return 0, "err completing request: " + err.Error()
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code, string(w.Body.Bytes())
}
