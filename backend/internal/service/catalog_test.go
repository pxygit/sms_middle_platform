package service

import (
	"encoding/json"
	"testing"

	"gorm.io/datatypes"
)

func TestNormalizeServiceConfigMetadata(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		metadata datatypes.JSON
		want     string
		wantErr  bool
	}{
		{name: "other provider unchanged", provider: "smspool", metadata: datatypes.JSON(`{"custom":true}`), want: ""},
		{name: "68sms defaults to virtual", provider: "68sms", want: "1"},
		{name: "68sms keeps physical", provider: "68sms", metadata: datatypes.JSON(`{"simType":"2"}`), want: "2"},
		{name: "68sms rejects unknown type", provider: "68sms", metadata: datatypes.JSON(`{"simType":"3"}`), wantErr: true},
		{name: "68sms rejects unknown numeric type", provider: "68sms", metadata: datatypes.JSON(`{"simType":3}`), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeServiceConfigMetadata(test.provider, test.metadata)
			if (err != nil) != test.wantErr {
				t.Fatalf("normalizeServiceConfigMetadata() error = %v, wantErr %v", err, test.wantErr)
			}
			if test.wantErr || test.provider != "68sms" {
				return
			}
			var values map[string]interface{}
			if err := json.Unmarshal(got, &values); err != nil {
				t.Fatal(err)
			}
			if values["simType"] != test.want {
				t.Fatalf("simType = %#v, want %q", values["simType"], test.want)
			}
		})
	}
}
