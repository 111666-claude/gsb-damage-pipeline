# damage-pipeline

战斗伤害结算管线：把一次攻击算成最终扣血，并留一份可核对的战斗日志。只用 Go 标准库。

```
go test ./...
go run ./cmd/damage-pipeline --sample armor
go run ./cmd/damage-pipeline --sample order
go run ./cmd/damage-pipeline --sample log
```

## 口径（README 为准）

- **结算顺序**：基础伤害 + 固定加成 → 暴击 → 百分比增伤 → 固定减伤 → 百分比抗性 → 护甲，
  顺序不能变（策划表口径）。`--sample order`（必定暴击 1.5、固定减伤 20、抗性 50%、基础 100）
  按口径是 65；现在把暴击放到最后算，给 45。
- **护甲公式对负护甲要增伤**：乘数 = `100 / (100 + armor)`，armor 允许为负（破甲），只要 armor > -100。
  `--sample armor`（基础 100、护甲 -50）按口径是 200，现在 `max(0, armor)` 把负护甲夹成 0，给 100。
- **只在最后取整**：中间步骤用浮点，最终向下取整；战斗日志里每一步的数字要能乘回最终伤害。
  `--sample log`（基础 41.5、抗性 10%、固定减伤 4）按口径是 33，现在每步都 floor，给 32 且日志对不上。
- 校验：基础伤害不能为负；抗性必须在 [0, 1)；护甲 ≤ -100 要报错；结算结果不能为负；
  同一条随机种子必须给出同样的结果（暴击判定走传入的 rng）。
- 不变量：日志里各步乘积与最终伤害一致（`consistent=true`）；同一输入重复结算结果相同；
  固定减伤不会被暴击放大。
- 规模：100 万次结算 1 秒内完成，单次 O(1)、不分配 map 或切片。

## 现在的行为

- `armor := math.Max(0, defender.Armor)`：负护甲不减反而不增伤，破甲形同虚设。
- 暴击放在流程最后：先被固定减伤和抗性削过再乘暴击，结果偏低；固定减伤也被暴击放大了。
- 每个中间步骤都 `math.Floor`，多次截断让伤害系统性偏低，日志里记的也是被截断的值，
  与「按策划公式算出来的中间值」对不上。

## 计划新增的能力

```
--sample armor|order|log   三个结算样例
--seed N                   暴击判定用的随机种子
```

## 输出

```
damage=200 consistent=true
step base=100.000
step armor=200.000
```

## 目录

```
pipeline.go            结算管线与日志
helpers.go             日志格式化
cmd/damage-pipeline    命令行入口
pipeline_test.go       go test 用例
```
