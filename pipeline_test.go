package damagepipe

import (
	"math/rand"
	"testing"
)

func fixed(seed int64) *rand.Rand {
	return rand.New(rand.NewSource(seed))
}

func TestNoCritNoMitigation(t *testing.T) {
	result, err := Compute(100, Attacker{}, Defender{}, fixed(1))
	if err != nil {
		t.Fatalf("Compute 出错：%v", err)
	}
	if result.Damage != 100 {
		t.Fatalf("没有任何减伤时应该原样：%d", result.Damage)
	}
}

func TestPositiveArmorReduces(t *testing.T) {
	result, _ := Compute(100, Attacker{}, Defender{Armor: 100}, fixed(1))
	if result.Damage >= 100 {
		t.Fatalf("正护甲应该减伤：%d", result.Damage)
	}
}

func TestCritIncreases(t *testing.T) {
	plain, _ := Compute(100, Attacker{}, Defender{}, fixed(1))
	crit, _ := Compute(100, Attacker{CritChance: 1, CritDamage: 2}, Defender{}, fixed(1))
	if crit.Damage <= plain.Damage {
		t.Fatalf("必定暴击应该更高：%d vs %d", crit.Damage, plain.Damage)
	}
}

func TestBadSpec(t *testing.T) {
	if _, err := Compute(-1, Attacker{}, Defender{}, fixed(1)); err != ErrSpec {
		t.Fatalf("负基础伤害应该报错：%v", err)
	}
	if _, err := Compute(100, Attacker{}, Defender{Resist: 1.5}, fixed(1)); err != ErrSpec {
		t.Fatalf("非法抗性应该报错：%v", err)
	}
}

func TestLogIsWritten(t *testing.T) {
	result, _ := Compute(50, Attacker{}, Defender{Armor: 50}, fixed(1))
	if len(result.Log) < 2 {
		t.Fatalf("应该留下日志：%v", result.Log)
	}
}

func TestSameInputSameResult(t *testing.T) {
	first, _ := Compute(77, Attacker{CritChance: 0.5, CritDamage: 1.5}, Defender{Armor: 12}, fixed(9))
	second, _ := Compute(77, Attacker{CritChance: 0.5, CritDamage: 1.5}, Defender{Armor: 12}, fixed(9))
	if first.Damage != second.Damage {
		t.Fatalf("同种子应该给同样结果：%d vs %d", first.Damage, second.Damage)
	}
}
