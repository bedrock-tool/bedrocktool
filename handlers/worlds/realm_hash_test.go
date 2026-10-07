package worlds

import "testing"

func TestRealmObservedBlockHash(t *testing.T) {
	br, err := newNetworkBlockRegistry(true)
	if err != nil {
		t.Fatal(err)
	}
	br.Finalize()
	rid, ok := br.HashToRuntimeID(2920631295)
	if !ok {
		t.Fatal("observed Realm hash 2920631295 missing from vanilla palette")
	}
	name, props, _ := br.RuntimeIDToState(rid)
	t.Logf("rid=%d state=%s %v", rid, name, props)
}
