package sms68

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestSIMTypeFromMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata json.RawMessage
		want     string
	}{
		{name: "missing defaults to virtual", want: simVirtual},
		{name: "physical", metadata: json.RawMessage(`{"simType":"2"}`), want: simPhysical},
		{name: "invalid defaults to virtual", metadata: json.RawMessage(`{"simType":"3"}`), want: simVirtual},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := simTypeFromMetadata(test.metadata); got != test.want {
				t.Fatalf("simTypeFromMetadata() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestValidityTypesPrioritizesConfiguredValue(t *testing.T) {
	metadata := json.RawMessage(`{"validityType":"2","simType":"2"}`)
	want := []string{"2", "4", "3", "1"}
	if got := validityTypes(metadata); !reflect.DeepEqual(got, want) {
		t.Fatalf("validityTypes() = %v, want %v", got, want)
	}
}

func TestMetadataScopes(t *testing.T) {
	client := &Client{}
	got := client.MetadataScopes()
	if len(got) != 2 || got[0].SIMType != simVirtual || got[1].SIMType != simPhysical {
		t.Fatalf("MetadataScopes() = %#v", got)
	}
}

func TestOperatorIDFromMetadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata json.RawMessage
		want     string
	}{
		{name: "virtual defaults to operator 2", metadata: json.RawMessage(`{"simType":"1"}`), want: "2"},
		{name: "physical defaults to operator 5", metadata: json.RawMessage(`{"simType":"2"}`), want: "5"},
		{name: "configured operator wins", metadata: json.RawMessage(`{"simType":"2","operatorId":"4"}`), want: "4"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := operatorIDFromMetadata(test.metadata); got != test.want {
				t.Fatalf("operatorIDFromMetadata() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestUpdateCommunication(t *testing.T) {
	client := &Client{metadataToken: "Token: token-value\nCookie: session=value\nCommunication: old-value"}
	var persisted string
	client.SetCommunicationUpdater(func(value, fallbackCredential string) error {
		persisted = value
		if !strings.Contains(fallbackCredential, "Communication: new-value") {
			t.Fatalf("fallback credential = %q", fallbackCredential)
		}
		return nil
	})

	client.updateCommunication("new-value")
	if persisted != "new-value" {
		t.Fatalf("persisted Communication = %q", persisted)
	}
	_, _, credentialText := client.config()
	credential := parseLoginCredential(credentialText)
	if credential.Token != "token-value" || credential.Cookie != "session=value" || credential.Communication != "new-value" {
		t.Fatalf("updated credential = %#v", credential)
	}

	persisted = ""
	client.updateCommunication("new-value")
	if persisted != "" {
		t.Fatal("unchanged Communication should not be persisted again")
	}
	client.updateCommunication("invalid\nheader")
	if strings.Contains(client.metadataToken, "invalid") {
		t.Fatal("invalid Communication header was accepted")
	}
}
