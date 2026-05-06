package businessnotification

import (
	"encoding/json"
	"testing"
)

func TestNormalizePayloadDefaultsCreatedAt(t *testing.T) {
	payload, err := NormalizePayload(Payload{
		BusinessType: string(ChannelProp),
		SubType:      PropSubTypeUse,
		Title:        "Prop used",
		Summary:      "A prop was used",
	})
	if err != nil {
		t.Fatalf("NormalizePayload returned error: %v", err)
	}
	if payload.CreatedAt == 0 {
		t.Fatal("CreatedAt should be defaulted")
	}
}

func TestNormalizePayloadRejectsUnsupportedPropSubtype(t *testing.T) {
	_, err := NormalizePayload(Payload{
		BusinessType: string(ChannelProp),
		SubType:      "bad_subtype",
		Title:        "Prop used",
		Summary:      "A prop was used",
	})
	if err == nil {
		t.Fatal("expected unsupported prop subtype error")
	}
}

func TestMarshalEnvelopeIncludesDeliveryFlags(t *testing.T) {
	detail, err := MarshalEnvelope(Payload{
		BusinessType: string(ChannelProp),
		SubType:      PropSubTypeExpire,
		Title:        "Prop expired",
		Summary:      "A prop expired",
		CreatedAt:    1,
	}, Delivery{
		CountUnread: false,
		OfflinePush: false,
	})
	if err != nil {
		t.Fatalf("MarshalEnvelope returned error: %v", err)
	}

	var envelope Envelope
	if err := json.Unmarshal([]byte(detail), &envelope); err != nil {
		t.Fatalf("detail is not valid envelope json: %v", err)
	}
	if envelope.Delivery.CountUnread {
		t.Fatal("CountUnread should be false")
	}
	if envelope.Delivery.OfflinePush {
		t.Fatal("OfflinePush should be false")
	}
	if envelope.Payload.SubType != PropSubTypeExpire {
		t.Fatalf("unexpected subtype: %s", envelope.Payload.SubType)
	}
}
