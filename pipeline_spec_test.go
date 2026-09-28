package damagepipe

import (
	"math"
	"testing"
	"time"
)

func mustCompute(t *testing.T, base float64, attacker Attacker, defender Defender, seed int64) Result {
	t.Helper()
	result, err := Compute(base, attacker, defender, fixed(seed))
	if err != nil {
		t.Fatalf("Compute 出错：%v", err)
	}
	return result
}

func TestNegativeArmorAmplifies(t *testing.T) {
	result := mustCompute(t, 100, Attacker{}, Defender{Armor: -50}, 1)
	if result.Damage != 200 {
		t.Fatalf("破甲 -50 应该双倍伤害：%d", result.Damage)
	}
	if !Consistent(result) {
		t.Fatalf("日志应该与扣血一致：%v", result.Log)
	}
}

func TestArmorBoundary(t *testing.T) {
	if _, err := Compute(100, Attacker{}, Defender{Armor: -100}, fixed(1)); err != ErrSpec {
		t.Fatalf("护甲 -100 应该报错：%v", err)
	}
	if _, err := Compute(100, Attacker{}, Defender{Armor: -100.5}, fixed(1)); err != ErrSpec {
		t.Fatalf("护甲低于 -100 应该报错：%v", err)
	}
	// 刚好大于 -100 是合法的。
	if _, err := Compute(100, Attacker{}, Defender{Armor: -99.999}, fixed(1)); err != nil {
		t.Fatalf("护甲略高于 -100 不该报错：%v", err)
	}
}

func TestResistBoundary(t *testing.T) {
	if _, err := Compute(100, Attacker{}, Defender{Resist: 1}, fixed(1)); err != ErrSpec {
		t.Fatalf("抗性 1 应该报错：%v", err)
	}
	if _, err := Compute(100, Attacker{}, Defender{Resist: -0.1}, fixed(1)); err != ErrSpec {
		t.Fatalf("负抗性应该报错：%v", err)
	}
	if _, err := Compute(100, Attacker{}, Defender{Resist: 0.999}, fixed(1)); err != nil {
		t.Fatalf("抗性 0.999 不该报错：%v", err)
	}
}

func TestPlannerOrderSample(t *testing.T) {
	// 策划表样例：必定暴击 1.5、固定减伤 20、抗性 50%、基础 100 → 65。
	result := mustCompute(t, 100, Attacker{CritChance: 1, CritDamage: 1.5}, Defender{FlatReduce: 20, Resist: 0.5}, 1)
	if result.Damage != 65 {
		t.Fatalf("结算顺序样例应该是 65：%d", result.Damage)
	}
	if !Consistent(result) {
		t.Fatalf("日志应该与扣血一致：%v", result.Log)
	}
}

func TestFlatReduceNotAmplifiedByCrit(t *testing.T) {
	// 固定减伤在暴击之后结算：(100*1.5)-20=130，而不是 (100-20)*1.5=120。
	result := mustCompute(t, 100, Attacker{CritChance: 1, CritDamage: 1.5}, Defender{FlatReduce: 20}, 1)
	if result.Damage != 130 {
		t.Fatalf("固定减伤不能被暴击放大：%d", result.Damage)
	}
}

func TestFlatBonusBeforeCrit(t *testing.T) {
	// 固定加成在暴击之前：(100+10)*1.5=165。
	result := mustCompute(t, 100, Attacker{CritChance: 1, CritDamage: 1.5, FlatBonus: 10}, Defender{}, 1)
	if result.Damage != 165 {
		t.Fatalf("固定加成应该吃暴击：%d", result.Damage)
	}
}

func TestSingleFloorAtEnd(t *testing.T) {
	// 基础 41.5、固定减伤 4、抗性 10%：(41.5-4)*0.9=33.75 → 33，中间不截断。
	result := mustCompute(t, 41.5, Attacker{}, Defender{Resist: 0.1, FlatReduce: 4}, 1)
	if result.Damage != 33 {
		t.Fatalf("应该只在最后取整：%d", result.Damage)
	}
	if !Consistent(result) {
		t.Fatalf("日志应该与扣血一致：%v", result.Log)
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
		{41.5, Attacker{}, Defender{Resist: 0.1, FlatReduce: 4}},
		{77, Attacker{CritChance: 0.5, CritDamage: 1.5}, Defender{Armor: 12}},
		{100, Attacker{FlatBonus: 10, DamageAmp: 0.2}, Defender{Armor: -30, Resist: 0.25, FlatReduce: 5}},
		{0, Attacker{}, Defender{}},
	}
	for i, tc := range cases {
		for seed := int64(0); seed < 20; seed++ {
			result := mustCompute(t, tc.base, tc.attacker, tc.defender, seed)
			if !Consistent(result) {
				t.Fatalf("用例 %d 种子 %d 日志不一致：%v damage=%d", i, seed, result.Log, result.Damage)
			}
			if result.Damage < 0 {
				t.Fatalf("用例 %d 种子 %d 伤害不能为负：%d", i, seed, result.Damage)
			}
		}
	}
}

func TestDamageNeverNegative(t *testing.T) {
	result := mustCompute(t, 10, Attacker{}, Defender{FlatReduce: 50}, 1)
	if result.Damage != 0 {
		t.Fatalf("固定减伤超过伤害时应该归零：%d", result.Damage)
	}
	if !Consistent(result) {
		t.Fatalf("归零后日志也要一致：%v", result.Log)
	}
}

func TestCritExpectation(t *testing.T) {
	// 暴击期望 = 基础×(1+暴击率×(倍率-1))，10 万次统计误差 ≤ 1%。
	const (
		base       = 100.0
		critChance = 0.3
		critDamage = 1.5
		trials     = 100000
	)
	rng := fixed(42)
	total := 0
	for i := 0; i < trials; i++ {
		result, err := Compute(base, Attacker{CritChance: critChance, CritDamage: critDamage}, Defender{}, rng)
		if err != nil {
			t.Fatalf("Compute 出错：%v", err)
		}
		total += result.Damage
	}
	got := float64(total) / trials
	want := base * (1 + critChance*(critDamage-1))
	if math.Abs(got-want)/want > 0.01 {
		t.Fatalf("暴击期望漂移：got %.4f want %.4f", got, want)
	}
}

func TestMillionSettlementsUnderOneSecond(t *testing.T) {
	rng := fixed(7)
	attacker := Attacker{CritChance: 0.3, CritDamage: 1.5}
	defender := Defender{Armor: -20, Resist: 0.1, FlatReduce: 3}
	start := time.Now()
	total := 0
	for i := 0; i < 1000000; i++ {
		result, err := Compute(100, attacker, defender, rng)
		if err != nil {
			t.Fatalf("Compute 出错：%v", err)
		}
		total += result.Damage
	}
	elapsed := time.Since(start)
	if elapsed > time.Second {
		t.Fatalf("100 万次结算超过 1 秒：%v", elapsed)
	}
	if total <= 0 {
		t.Fatalf("结果不应该全零：%d", total)
	}
}
