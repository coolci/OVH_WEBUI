package db

import (
	"encoding/json"
	"fmt"
	"strings"
)

// decodeJSONColumn 解码 JSON 列;空串视为"未设置"(老库可能存在),其余解析失败一律报错。
//
// 以前这些列解析失败被 `_ =` 吞掉,得到空切片/空 map。而空的机房、配置、下单账户
// 在业务里都表示"不限" —— 一条损坏的记录会变成"任意机房、任意配置"去下单;
// 空的库存基准会让持续有货被当成首次补货再下一单。读取失败时整表加载失败,
// 由 app 层的 MarkLoadFailed 禁止覆盖写,等人工恢复(移植自上游 v0.4.14)。
func decodeJSONColumn(raw string, v interface{}, what string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	if err := json.Unmarshal([]byte(raw), v); err != nil {
		return fmt.Errorf("%s: 记录已损坏,拒绝按空值继续: %w", what, err)
	}
	return nil
}
