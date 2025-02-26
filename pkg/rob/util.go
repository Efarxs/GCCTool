package rob

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// 内部工具函数

// getPostDataMap 从 HTML 内容中提取 <input> 标签的 name 和 value 属性，并存储到 map 中
func getPostDataMap(res string, dataMap map[string]string) map[string]string {
	// 正则表达式匹配 <input> 标签
	inputPattern := regexp.MustCompile(`<input(.*?)\/>`)
	inputMatches := inputPattern.FindAllStringSubmatch(res, -1)

	// 遍历所有匹配的 <input> 标签
	for _, inputMatch := range inputMatches {
		if len(inputMatch) < 2 {
			continue
		}
		item := inputMatch[1]

		// 正则表达式匹配 name 和 value 属性
		attrPattern := regexp.MustCompile(`name="(.*?)".*?value="(.*?)"`)
		attrMatches := attrPattern.FindStringSubmatch(item)

		if len(attrMatches) >= 3 {
			name := attrMatches[1]
			value := attrMatches[2]
			dataMap[name] = value
		}
	}

	return dataMap
}

func map2String(dataMap map[string]string) string {
	var postData = strings.Builder{}
	for k, v := range dataMap {
		postData.WriteString(fmt.Sprintf("%s=%s&", k, v))
	}
	// 去掉最后一个&
	return postData.String()[:postData.Len()-1]
}

func timeClock(hour, minute int) bool {

	// 1. 构造今天的指定时间
	today := time.Now()
	specifiedTime := time.Date(today.Year(), today.Month(), today.Day(), hour, minute, 0, 0, today.Location())

	// 2. 获取当前时间
	currentTime := time.Now()

	// 3. 比较当前时间是否超过指定时间
	if currentTime.After(specifiedTime) || currentTime.Equal(specifiedTime) {
		return true
	}
	return false
}

func mustAtoi(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

func indexOf(element string, data []string) int {
	for k, v := range data {
		if element == v {
			return k
		}
	}
	return -1
}

func getNow() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// 是否在列表中
func contains(element string, data []string) bool {
	return indexOf(element, data) > -1
}

// toInt
func toInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

func toFloat(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

func getString(s interface{}) string {
	if s == nil {
		return ""
	}
	if r, ok := s.(string); ok {
		return r
	}
	return ""
}

func getInt(s interface{}) int {
	if s == nil {
		return 0
	}
	if r, ok := s.(int); ok {
		return r
	}
	if r := getString(s); r != "" {
		return toInt(r)
	}
	return 0
}
