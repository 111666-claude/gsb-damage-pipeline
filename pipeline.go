// Package damagepipe 是战斗伤害结算管线：把一次攻击算成最终扣血并留一份战斗日志。
// 结算顺序按策划表：基础 + 固定加成 → 暴击 → 百分比增伤 → 固定减伤 → 百分比抗性 → 护甲，
// 中间步骤一律浮点，只在最后向下取整；日志记录的就是真实扣血路径上的值。
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
	FlatBonus  float64 // 固定加成，结算第一步加到基础伤害上
	DamageAmp  float64 // 百分比增伤，0.2 表示 +20%
}

// Defender 是防守方参数。
type Defender struct {
	Armor      float64 // 可以为负（破甲）
	Resist     float64 // 0..1 的百分比抗性
	FlatReduce float64 // 固定减伤
}

// Result 是一次结算的结果。
type Result struct {
	Damage int
	Log    []string
	final  float64 // 取整前的真实扣血，供 Consistent 核对
}

// Compute 结算一次伤害。
// 顺序固定为：基础 + 固定加成 → 暴击 → 百分比增伤 → 固定减伤 → 百分比抗性 → 护甲。
// 护甲乘数 = 100/(100+armor)，armor 允许为负（破甲增伤），armor <= -100 报错。
// 中间步骤用浮点，只在最后向下取整；日志只记录实际改变数值的步骤。
func Compute(base float64, attacker Attacker, defender Defender, rng *rand.Rand) (Result, error) {
	if base < 0 || defender.Resist < 0 || defender.Resist >= 1 || defender.Armor <= -100 {
		return Result{}, ErrSpec
	}
	crit := rng.Float64() < attacker.CritChance

	value := base + attacker.FlatBonus

	// 日志与扣血共用同一条路径：定长数组记录步骤，不分配 map，切片只是数组的视图。
	var steps [7]string
	count := 0
	steps[count] = format("base", value)
	count++
	if crit {
		value *= attacker.CritDamage
		steps[count] = format("crit", value)
		count++
	}
	if attacker.DamageAmp != 0 {
		value *= 1 + attacker.DamageAmp
		steps[count] = format("amp", value)
		count++
	}
	if defender.FlatReduce != 0 {
		value = math.Max(0, value-defender.FlatReduce) // 固定减伤在暴击之后，不会被暴击放大
		steps[count] = format("flat", value)
		count++
	}
	if defender.Resist != 0 {
		value *= 1 - defender.Resist
		steps[count] = format("resist", value)
		count++
	}
	if defender.Armor != 0 {
		value *= 100 / (100 + defender.Armor)
		steps[count] = format("armor", value)
		count++
	}

	damage := int(math.Floor(value)) // 只在最后取整
	if damage < 0 {
		damage = 0 // 伤害不得为负
	}
	return Result{Damage: damage, Log: steps[:count], final: value}, nil
}

func format(step string, value float64) string {
	return step + "=" + trim(value)
}

func trim(value float64) string {
	return strconvFloat(value)
}

// Consistent 是诊断接口：核对日志最后一步与真实扣血是否一致。
// 日志按 3 位小数格式化，允许半个格式化单位以内的误差。
func Consistent(result Result) bool {
	if len(result.Log) == 0 {
		return false
	}
	last := parseLast(result.Log[len(result.Log)-1])
	if last < 0 {
		return false
	}
	return math.Abs(last-result.final) <= 0.0005 &&
		math.Floor(result.final) == float64(result.Damage)
}
