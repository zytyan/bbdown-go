package bili

import "testing"

func TestBVAVRoundTrip(t *testing.T) {
	const aid int64 = 170001
	bv := AVToBV(aid)
	got, err := BvToAV(bv)
	if err != nil {
		t.Fatal(err)
	}
	if got != aid {
		t.Fatalf("BvToAV(AVToBV(%d)) = %d", aid, got)
	}
}

func TestDecodeTargetBV(t *testing.T) {
	aid, err := BvToAV("BV13TKS6VE4Y")
	if err != nil {
		t.Fatal(err)
	}
	if aid <= 0 {
		t.Fatalf("aid should be positive: %d", aid)
	}
	if got := AVToBV(aid); got != "BV13TKS6VE4Y" {
		t.Fatalf("round trip = %s", got)
	}
}

func TestParseAID(t *testing.T) {
	id, err := ParseInputID("https://www.bilibili.com/video/BV17x411w7KC")
	if err != nil {
		t.Fatal(err)
	}
	if id.AID == 0 || id.BVID != "BV17x411w7KC" || id.FileName != "BV17x411w7KC" {
		t.Fatalf("unexpected parse result: %+v", id)
	}
}

func TestParseInputIDKeepsAVName(t *testing.T) {
	id, err := ParseInputID("https://www.bilibili.com/video/av170001")
	if err != nil {
		t.Fatal(err)
	}
	if id.AID != 170001 || id.FileName != "av170001" {
		t.Fatalf("unexpected parse result: %+v", id)
	}
}
