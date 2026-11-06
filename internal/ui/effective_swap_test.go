package ui

import "testing"

func TestModelEffectiveSwapPanes(t *testing.T) {
	forced := func(m *Model) { // quick view on a carousel driver with ordering on
		m.CarouselQuickViewOrdering = true
		m.QuickViewEnabled = true
	}
	tests := []struct {
		name  string
		setup func(m *Model)
		want  func(pref bool) bool // result for the given preference
	}{
		{"nothing set", func(m *Model) {}, func(p bool) bool { return p }},
		{"ordering off", func(m *Model) {
			forced(m)
			m.CarouselQuickViewOrdering = false
			m.QuickViewPanel = SecondaryPanel
			m.Secondary.CarouselMode = true
		}, func(p bool) bool { return p }},
		{"quick view off", func(m *Model) {
			forced(m)
			m.QuickViewEnabled = false
			m.QuickViewPanel = SecondaryPanel
			m.Secondary.CarouselMode = true
		}, func(p bool) bool { return p }},
		{"driver unset", func(m *Model) {
			forced(m)
			m.QuickViewPanel = -1
			m.Primary.CarouselMode, m.Secondary.CarouselMode = true, true
		}, func(p bool) bool { return p }},
		{"primary driver not carousel", func(m *Model) {
			forced(m)
			m.QuickViewPanel = PrimaryPanel
			m.Secondary.CarouselMode = true
		}, func(p bool) bool { return p }},
		{"secondary driver not carousel", func(m *Model) {
			forced(m)
			m.QuickViewPanel = SecondaryPanel
			m.Primary.CarouselMode = true
		}, func(p bool) bool { return p }},
		{"primary driver carousel", func(m *Model) {
			forced(m)
			m.QuickViewPanel = PrimaryPanel
			m.Primary.CarouselMode = true
		}, func(bool) bool { return false }},
		{"secondary driver carousel", func(m *Model) {
			forced(m)
			m.QuickViewPanel = SecondaryPanel
			m.Secondary.CarouselMode = true
		}, func(bool) bool { return true }},
		{"both carousel, primary driver", func(m *Model) {
			forced(m)
			m.QuickViewPanel = PrimaryPanel
			m.Primary.CarouselMode, m.Secondary.CarouselMode = true, true
		}, func(bool) bool { return false }},
	}
	for _, tc := range tests {
		for _, pref := range []bool{false, true} {
			var m Model
			m.SwapPanes = pref
			tc.setup(&m)
			if got, want := m.EffectiveSwapPanes(), tc.want(pref); got != want {
				t.Errorf("%s pref=%v: EffectiveSwapPanes = %v, want %v", tc.name, pref, got, want)
			}
			first := PrimaryPanel
			if m.EffectiveSwapPanes() {
				first = SecondaryPanel
			}
			if m.FirstSlotPanel() != first || m.SecondSlotPanel() != PrimaryPanel+SecondaryPanel-first {
				t.Errorf("%s pref=%v: slots %d/%d, want first %d", tc.name, pref, m.FirstSlotPanel(), m.SecondSlotPanel(), first)
			}
		}
	}
}
