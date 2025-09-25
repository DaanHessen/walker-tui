package engine

import "testing"

func TestCustomGating_FatigueCritical(t *testing.T) {
    s := Survivor{Stats: Stats{Fatigue: 90}}
    if _, ok, _ := ValidateCustomAction("forage the area", s); ok {
        t.Fatalf("expected custom action denied when fatigue>85 and not rest")
    }
    if _, ok, _ := ValidateCustomAction("rest now", s); !ok {
        t.Fatalf("expected rest allowed even when fatigue high")
    }
}

func TestCustomGating_NeedsCritical(t *testing.T) {
    s := Survivor{Stats: Stats{Hunger: 96, Thirst: 50}}
    if _, ok, _ := ValidateCustomAction("organize supplies", s); ok {
        t.Fatalf("expected non-forage denied when hunger critical")
    }
    s = Survivor{Stats: Stats{Hunger: 50, Thirst: 96}}
    if _, ok, _ := ValidateCustomAction("barricade the door", s); ok {
        t.Fatalf("expected non-forage denied when thirst critical")
    }
    s = Survivor{Stats: Stats{Hunger: 96, Thirst: 96}}
    if _, ok, _ := ValidateCustomAction("forage the area", s); !ok {
        t.Fatalf("expected forage allowed at critical needs")
    }
}

