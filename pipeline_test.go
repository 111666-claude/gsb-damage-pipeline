package damagepipe

import (
	"math"
	"math/rand"
	"testing"
	"time"
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

func TestNegativeArmorAmplifies(t *testing.T) {
	result, err := Compute(100, Attacker{}, Defender{Armor: -50}, fixed(1))
	if err != nil {
		t.Fatalf("破甲不该报错：%v", err)
	}
	if result.Damage != 200 {
		t.Fatalf("护甲 -50 应该增伤到 200：%d", result.Damage)
	}
	if !Consistent(result) {
		t.Fatalf("日志应该与扣血一致：%v", result.Log)
	}
}

func TestArmorBoundary(t *testing.T) {
	if _, err := Compute(100, Attacker{}, Defender{Armor: -100}, fixed(1)); err != ErrSpec {
		t.Fatalf("护甲 -100 应该报错：%v", err)
	}
	if _, err := Compute(100, Attacker{}, Defender{Armor: -200}, fixed(1)); err != ErrSpec {
		t.Fatalf("护甲低于 -100 应该报错：%v", err)
	}
	if _, err := Compute(100, Attacker{}, Defender{Armor: -99.999}, fixed(1)); err != nil {
		t.Fatalf("护甲略高于 -100 不该报错：%v", err)
	}
}

func TestPipelineOrder(t *testing.T) {
	// 必定暴击 1.5、固定减伤 20、抗性 50%、基础 100：
	// ((100*1.5)-20)*0.5 = 65，顺序错了会算出别的值。
	result, err := Compute(100, Attacker{CritChance: 1, CritDamage: 1.5},
		Defender{FlatReduce: 20, Resist: 0.5}, fixed(1))
	if err != nil {
		t.Fatalf("Compute 出错：%v", err)
	}
	if result.Damage != 65 {
		t.Fatalf("结算顺序应为 基础→暴击→增伤→固定减伤→抗性→护甲，得 65：%d", result.Damage)
	}
	if !Consistent(result) {
		t.Fatalf("日志应该与扣血一致：%v", result.Log)
	}
}

func TestFlatReduceNotAmplifiedByCrit(t *testing.T) {
	// 固定减伤在暴击之后结算：100*2-20=180，而不是 (100-20)*2=160。
	result, _ := Compute(100, Attacker{CritChance: 1, CritDamage: 2},
		Defender{FlatReduce: 20}, fixed(1))
	if result.Damage != 180 {
		t.Fatalf("固定减伤不能被暴击放大，应为 180：%d", result.Damage)
	}
}

func TestFlatBonusAndAmpOrder(t *testing.T) {
	// (100+10)*1.5*1.2 = 198：固定加成最先进，百分比增伤在暴击之后。
	result, _ := Compute(100, Attacker{CritChance: 1, CritDamage: 1.5, FlatBonus: 10, DamageAmp: 0.2},
		Defender{}, fixed(1))
	if result.Damage != 198 {
		t.Fatalf("固定加成与百分比增伤顺序不对，应为 198：%d", result.Damage)
	}
}

func TestFloorOnlyAtEnd(t *testing.T) {
	// 基础 41.5、固定减伤 4、抗性 10%：(41.5-4)*0.9=33.75，最后取整得 33；
	// 每步都 floor 会得到 32。
	result, err := Compute(41.5, Attacker{}, Defender{Resist: 0.10, FlatReduce: 4}, fixed(1))
	if err != nil {
		t.Fatalf("Compute 出错：%v", err)
	}
	if result.Damage != 33 {
		t.Fatalf("中间步骤不该取整，应为 33：%d", result.Damage)
	}
	if !Consistent(result) {
		t.Fatalf("日志应该与扣血一致：%v", result.Log)
	}
	last := result.Log[len(result.Log)-1]
	if last != "resist=33.750" {
		t.Fatalf("日志末尾应是未取整的 33.750：%q", last)
	}
}

func TestLogConsistentAcrossSamples(t *testing.T) {
	cases := []struct {
		base     float64
		attacker Attacker
		defender Defender
	}{
		{100, Attacker{}, Defender{Armor: -50}},
		{100, Attacker{CritChance: 1, CritDamage: 1.5}, Defender{FlatReduce: 20, Resist: 0.5}},
		{41.5, Attacker{}, Defender{Resist: 0.10, FlatReduce: 4}},
		{77, Attacker{CritChance: 0.5, CritDamage: 1.5, FlatBonus: 3, DamageAmp: 0.1},
			Defender{Armor: 12, Resist: 0.25, FlatReduce: 5}},
	}
	for i, c := range cases {
		result, err := Compute(c.base, c.attacker, c.defender, fixed(int64(i+1)))
		if err != nil {
			t.Fatalf("样例 %d 出错：%v", i, err)
		}
		if !Consistent(result) {
			t.Fatalf("样例 %d 日志与扣血不一致：%v damage=%d", i, result.Log, result.Damage)
		}
	}
}

func TestDamageNeverNegative(t *testing.T) {
	result, err := Compute(10, Attacker{}, Defender{FlatReduce: 1000}, fixed(1))
	if err != nil {
		t.Fatalf("Compute 出错：%v", err)
	}
	if result.Damage != 0 {
		t.Fatalf("固定减伤超过伤害时应为 0：%d", result.Damage)
	}
	if !Consistent(result) {
		t.Fatalf("被减到 0 时日志也要一致：%v", result.Log)
	}
}

func TestBadSpecBoundaries(t *testing.T) {
	if _, err := Compute(0, Attacker{}, Defender{}, fixed(1)); err != nil {
		t.Fatalf("基础伤害 0 应该合法：%v", err)
	}
	if _, err := Compute(100, Attacker{}, Defender{Resist: 0}, fixed(1)); err != nil {
		t.Fatalf("抗性 0 应该合法：%v", err)
	}
	if _, err := Compute(100, Attacker{}, Defender{Resist: 0.999}, fixed(1)); err != nil {
		t.Fatalf("抗性接近 1 应该合法：%v", err)
	}
	if _, err := Compute(100, Attacker{}, Defender{Resist: 1}, fixed(1)); err != ErrSpec {
		t.Fatalf("抗性 1 应该报错：%v", err)
	}
	if _, err := Compute(100, Attacker{}, Defender{Resist: -0.1}, fixed(1)); err != ErrSpec {
		t.Fatalf("负抗性应该报错：%v", err)
	}
}

func TestCritExpectation(t *testing.T) {
	const base = 100.0
	const chance = 0.3
	const mult = 1.5
	const n = 100000
	rng := fixed(20260929)
	sum := 0.0
	for i := 0; i < n; i++ {
		result, err := Compute(base, Attacker{CritChance: chance, CritDamage: mult}, Defender{}, rng)
		if err != nil {
			t.Fatalf("Compute 出错：%v", err)
		}
		sum += float64(result.Damage)
	}
	got := sum / n
	want := base * (1 + chance*(mult-1))
	if math.Abs(got-want)/want > 0.01 {
		t.Fatalf("暴击期望偏差超过 1%%：got %.4f want %.4f", got, want)
	}
}

func TestMillionSettlementsUnderOneSecond(t *testing.T) {
	rng := fixed(42)
	attacker := Attacker{CritChance: 0.3, CritDamage: 1.5, FlatBonus: 5, DamageAmp: 0.1}
	defender := Defender{Armor: 30, Resist: 0.2, FlatReduce: 5}
	start := time.Now()
	sum := 0
	for i := 0; i < 1000000; i++ {
		result, err := Compute(100, attacker, defender, rng)
		if err != nil {
			t.Fatalf("Compute 出错：%v", err)
		}
		sum += result.Damage
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("100 万次结算超过 1 秒：%v", elapsed)
	}
	if sum <= 0 {
		t.Fatalf("结算结果异常：%d", sum)
	}
}
