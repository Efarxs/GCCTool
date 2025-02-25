package errs

import "errors"

var (
	ErrLoginExpired = errors.New("登录失效了")

	ErrPassword = errors.New("学号或密码错误")

	ErrClassList = errors.New("获取课程列表失败")

	ErrDataInit = errors.New("数据初始化失败")

	ErrRobTime = errors.New("初始化课程信息失败，当前不属于抢课阶段")

	ErrLoginFail = errors.New("二次登录失败")

	ErrServerFail = errors.New("网络异常")

	ErrInitClassInfo = errors.New("初始化课表失败")

	ErrClassInfo = errors.New("获取课程信息失败")
)
