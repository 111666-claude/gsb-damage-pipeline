// Package damagepipe 是战斗伤害结算管线：把一次攻击算成最终扣血并留一份战斗日志。
// 缺陷：护甲用 max(0, armor) 夹住（负护甲不减伤）、结算顺序与策划表相反、每一步都向下取整。
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
}

// Compute 结算一次伤害。
// 缺陷：实际扣血走「护甲（夹到 >= 0）→ 抗性 → 固定减伤 → 暴击」并在每步 floor；
// 战斗日志却按策划口径的另一条路径算（暴击在前、不减夹护甲、只在最后取整），两条路会漂移。
func Compute(base float64, attacker Attacker, defender Defender, rng *rand.Rand) (Result, error) {
	if base < 0 || defender.Resist < 0 || defender.Resist >= 1 {
		return Result{}, ErrSpec
	}
	crit := rng.Float64() < attacker.CritChance

	// 实际扣血：缺陷路径
	damage := base
	armor := math.Max(0, defender.Armor) // 缺陷 1：负护甲被夹成 0
	damage = math.Floor(damage * 100 / (100 + armor))
	damage = math.Floor(damage * (1 - defender.Resist))
	damage = math.Max(0, damage-defender.FlatReduce)
	if crit {
		damage = math.Floor(damage * attacker.CritDamage) // 缺陷 2：暴击放在最后
	}

	// 战斗日志：策划口径的那条路径
	log := []string{}
	ideal := base
	log = append(log, format("base", ideal))
	if crit {
		ideal *= attacker.CritDamage
		log = append(log, format("crit", ideal))
	}
	ideal = math.Max(0, ideal-defender.FlatReduce)
	log = append(log, format("flat", ideal))
	ideal *= 1 - defender.Resist
	log = append(log, format("resist", ideal))
	ideal *= 100 / (100 + defender.Armor)
	log = append(log, format("armor", ideal))
	return Result{Damage: int(damage), Log: log}, nil
}

func format(step string, value float64) string {
	return step + "=" + trim(value)
}

func trim(value float64) string {
	return strconvFloat(value)
}

// Consistent 是诊断接口：把日志最后一步（策划口径的值）与真实扣血比一比。
func Consistent(result Result) bool {
	if len(result.Log) == 0 {
		return false
	}
	return math.Floor(parseLast(result.Log[len(result.Log)-1])) == float64(result.Damage)
}
