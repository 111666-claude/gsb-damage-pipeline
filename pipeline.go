// Package damagepipe 是战斗伤害结算管线：把一次攻击算成最终扣血并留一份战斗日志。
// 结算顺序（策划表口径）：基础 + 固定加成 → 暴击 → 百分比增伤 → 固定减伤 → 百分比抗性 → 护甲。
// 中间步骤一律浮点，只在最后向下取整；日志记录的就是真实扣血路径上的中间值。
package damagepipe

import (
	"errors"
	"math"
	"math/rand"
)

// ErrSpec 表示结算参数不合法。
var ErrSpec = errors.New("damagepipe: 结算参数不合法")

// Attacker 是攻击方参数。
type Attacker struct {
	CritChance float64 // 0..1
	CritDamage float64 // 1.5 表示 150%
	FlatBonus  float64 // 固定加成，在暴击之前结算
	DamageAmp  float64 // 百分比增伤，0.2 表示 +20%
}

// Defender 是防守方参数。
type Defender struct {
	Armor      float64 // 可以为负（破甲），但必须 > -100
	Resist     float64 // [0, 1) 的百分比抗性
	FlatReduce float64 // 固定减伤，在暴击之后结算，不会被暴击放大
}

// Result 是一次结算的结果。
type Result struct {
	Damage int
	Log    []string
}

// Compute 结算一次伤害。
// 单条路径：基础 + 固定加成 → 暴击 → 百分比增伤 → 固定减伤 → 百分比抗性 → 护甲，
// 日志逐步记录这条路径上的浮点中间值，最终伤害就是对末尾值向下取整。
func Compute(base float64, attacker Attacker, defender Defender, rng *rand.Rand) (Result, error) {
	if base < 0 || defender.Resist < 0 || defender.Resist >= 1 || defender.Armor <= -100 {
		return Result{}, ErrSpec
	}
	crit := rng.Float64() < attacker.CritChance

	log := make([]string, 0, 8)
	damage := base
	log = append(log, format("base", damage))
	if attacker.FlatBonus != 0 {
		damage = math.Max(0, damage+attacker.FlatBonus)
		log = append(log, format("bonus", damage))
	}
	if crit {
		damage *= attacker.CritDamage
		log = append(log, format("crit", damage))
	}
	if attacker.DamageAmp != 0 {
		damage *= 1 + attacker.DamageAmp
		log = append(log, format("amp", damage))
	}
	if defender.FlatReduce != 0 {
		damage = math.Max(0, damage-defender.FlatReduce)
		log = append(log, format("flat", damage))
	}
	if defender.Resist != 0 {
		damage *= 1 - defender.Resist
		log = append(log, format("resist", damage))
	}
	if defender.Armor != 0 {
		damage *= 100 / (100 + defender.Armor)
		log = append(log, format("armor", damage))
	}
	return Result{Damage: int(math.Floor(damage)), Log: log}, nil
}

func format(step string, value float64) string {
	return step + "=" + trim(value)
}

func trim(value float64) string {
	return strconvFloat(value)
}

// Consistent 是诊断接口：把日志最后一步与真实扣血比一比。
func Consistent(result Result) bool {
	if len(result.Log) == 0 {
		return false
	}
	return math.Floor(parseLast(result.Log[len(result.Log)-1])) == float64(result.Damage)
}
