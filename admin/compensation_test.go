package admin

import "testing"

func TestParseCompensationItemsRejectsInvalidQuantity(t *testing.T) {
	if _, err := parseCompensationItems("零食*not-a-number"); err == nil {
		t.Fatal("parseCompensationItems accepted an invalid quantity")
	}
}

func TestParseCompensationItemsParsesMultipleItems(t *testing.T) {
	items, err := parseCompensationItems("零食*2#抽奖券")
	if err != nil || len(items) != 2 || items[0].Quantity != 2 || items[1].Quantity != 1 {
		t.Fatalf("parseCompensationItems() = %#v, %v", items, err)
	}
}
