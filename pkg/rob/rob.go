package rob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"jwxt/model"
	"jwxt/pkg/component"
	"jwxt/pkg/errs"
	"jwxt/pkg/logger"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Robber struct {
	lock       sync.RWMutex
	Success    atomic.Bool
	loginRes   atomic.Bool
	isBug      atomic.Bool
	jwxt       *JwxtRob
	config     model.Config
	Ctx        context.Context
	CancelFunc context.CancelFunc
	Logger     *logger.Logger
	ui         *model.UIComponents
	myWindow   fyne.Window

	threadNum int
}

func NewRobber(myWindow fyne.Window, ui *model.UIComponents, logger2 *logger.Logger) *Robber {
	return &Robber{
		Logger:   logger2,
		ui:       ui,
		myWindow: myWindow,
	}
}

func (r *Robber) StartRob(ui model.UIComponents) {
	ctx, cancel := context.WithCancel(context.Background())
	r.Ctx = ctx
	r.CancelFunc = cancel

	var selectedCourseTypes []string
	for i, checkBox := range ui.CheckBoxes {
		if checkBox.Checked {
			selectedCourseTypes = append(selectedCourseTypes, strconv.Itoa(i+1))
		}
	}

	fmt.Println(selectedCourseTypes)
	selectedRadioButton := ui.RadioButtonGroup.Selected
	selectedComboBoxIndex := ui.ComboBox.SelectedIndex()
	url := []string{"172.22.14.1", "172.22.14.2", "172.22.14.4", "210.39.240.133", "jwxt.gcc.edu.cn", "172.22.14.3", "172.22.14.8"}[selectedComboBoxIndex]
	var courseNumList []string
	if len(strings.TrimSpace(ui.CourseNumListEntry.Text)) > 0 {
		courseNumList = strings.Split(ui.CourseNumListEntry.Text, ",")
	}

	threadNum, err := strconv.Atoi(ui.ThreadNumEntry.Text)
	if err != nil {
		dialog.ShowInformation("提示", "线程数不合法", r.myWindow)
		r.stopEvent()
		return
	}

	if threadNum < 1 {
		threadNum = 1
	}

	aHeadMinute := toInt(ui.AHeadMinuteEntry.Text)
	r.threadNum = threadNum

	r.config = model.Config{
		URL:           url,
		Account:       ui.AccountEntry.Text,
		Password:      ui.PasswordEntry.Text,
		Time:          ui.HourEntry.Text + ":" + ui.MinuteEntry.Text,
		Hour:          ui.HourEntry.Text,
		Minute:        ui.MinuteEntry.Text,
		MinCredit:     mustAtoi(ui.MinCreditEntry.Text),
		CourseType:    indexOf(selectedRadioButton, []string{"普通网课", "体育课", "普通课"}),
		CourseName:    ui.CourseNameEntry.Text,
		TeacherName:   ui.TeacherEntry.Text,
		CourseNumList: courseNumList,
		AHeadMinute:   aHeadMinute,
		AgentUrl:      ui.AgentEntry.Text,
		CategoryList:  selectedCourseTypes,
	}

	if r.config.Account == "" {
		dialog.ShowInformation("提示", "请填写学号", r.myWindow)
		r.stopEvent()
		return
	}
	if r.config.Password == "" {
		dialog.ShowInformation("提示", "请填写密码", r.myWindow)
		r.stopEvent()
		return
	}

	// 加载data.json
	file, err := os.ReadFile("./data.json")
	if err != nil {
		dialog.ShowInformation("提示", "加载data.json失败", r.myWindow)
		r.stopEvent()
		return
	}

	// 解析成 map[string][]string
	var data map[string][]string
	err = json.Unmarshal(file, &data)
	if err != nil {
		dialog.ShowInformation("提示", "解析data.json失败", r.myWindow)
		r.stopEvent()
		return
	}

	r.jwxt = NewJwxtRob(r.config, r.Logger, data)

	maxRetries := aHeadMinute * 50

	go r.start(r.threadNum, maxRetries)
}

func (r *Robber) StopRob() {
	if r.CancelFunc != nil {
		r.CancelFunc()
		r.CancelFunc = nil
	}
}

func (r *Robber) worker(taskChan <-chan map[string]interface{}, wg *sync.WaitGroup, thread int) {
	//defer func() {
	//	wg.Done()
	//	fmt.Printf("wgc%d\n", thread)
	//}()
	defer wg.Done()
	//fmt.Printf("wgs%d\n", thread)
	for class := range taskChan {
		if r.Ctx.Err() != nil {
			r.formatLog(fmt.Sprintf("任务被中止-线程[%d]", thread))
			return
		}
		result := r.executeClassTask(class, thread)
		if !result {
			continue
		}
		return
	}
}

func (r *Robber) executeClassTask(class map[string]interface{}, thread int) bool {
	classInfo, err := r.jwxt.GetClassInfo(class)
	if err != nil || len(classInfo) == 0 {
		return false
	}

	if r.doSelect(classInfo, class, thread) == 1 {
		return true // 抢课成功
	}
	return false // 抢课失败
}

func (r *Robber) isLogin(hour, minute int) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	today := time.Now()
	for {
		// 检查 context 是否已取消
		select {
		case <-r.Ctx.Done():
			return nil
		case <-ticker.C:
			if !timeClock(hour, minute) {
				r.formatLog("抢课时间没到[" + time.Date(today.Year(), today.Month(), today.Day(), hour, minute, 0, 0, today.Location()).Format("2006-01-02 15:04:05") + "]")
				// 维持ck
				_, err := r.jwxt.IsLogin()
				if err != nil {
					return err
				}
			} else {
				return nil
			}
		}

	}
}

// start 核心启动方法 并发线程数 重试数
func (r *Robber) start(concurrentLimit, maxRetries int) {
	r.isBug.Store(false)
	r.Success.Store(false)

	hour := toInt(r.config.Hour)
	minute := toInt(r.config.Minute)

	aHeadMinute := r.config.AHeadMinute
	if aHeadMinute > 0 {
		aHeadHour := aHeadMinute / 60
		minute = minute - (aHeadMinute % 60)
		if minute < 0 {
			minute = 0
		}
		if aHeadHour > 0 {
			hour = hour - aHeadHour
		}
		if hour < 0 {
			hour = 0
		}
	}

	for {
		// 检查 context 是否已取消
		select {
		case <-r.Ctx.Done():
			r.stopEvent()
			return
		default:
		}

		// step1. 用户登录
		if !r.loginRes.Load() {
			err := r.login()
			if err != nil {
				if errors.Is(err, errs.ErrPassword) {
					// 密码错误
					r.formatLog("因密码错误而中止所有任务")
				} else if errors.Is(err, errs.ErrServerFail) {
					// 服务器错误 网络问题
					r.formatLog("网络连接失败，请手动重试")
				} else {
					r.formatLog(err.Error())
				}
				r.StopRob()
				r.stopEvent()
				break
			} else {
				r.loginRes.Store(true)
				r.formatLog("登录成功，任务开始...")
			}
		}

		// step2. 判断时间
		err := r.isLogin(hour, minute)
		if err != nil {
			if errors.Is(err, errs.ErrLoginExpired) {
				r.loginRes.Store(false)
			} else if errors.Is(err, errs.ErrServerFail) {
				r.formatLog("网络连接失败，请手动重试")
			}
			continue
		}

		// step3. 获取课表
		// 重置err
		err = nil

		var wg sync.WaitGroup
		// 重试机制
		var classList []map[string]interface{}
		// 尝试获取课程列表，最多重试 maxRetries 次
		for retries := 0; retries < maxRetries; retries++ {
			select {
			case <-r.Ctx.Done():
				r.stopEvent()
				return
			default:
				classList, err = r.jwxt.GetClassList()
				if err != nil {
					// 打印错误并重试
					r.formatLog(fmt.Sprintf("获取课程列表失败: %v, 正在重试 %d/%d", err, retries+1, maxRetries))
					time.Sleep(2 * time.Second)
					continue
				}
			}
			if len(classList) > 0 {
				break
			}
		}

		taskChan := make(chan map[string]interface{}, concurrentLimit+2)

		// 如果重试后仍然失败，返回错误
		if err != nil {
			r.formatLog("获取课程列表失败，已达到最大重试次数")
			close(taskChan)
			r.StopRob()
			r.stopEvent()
			return
		}

		// 启动任务分发
		go func() {
			for _, class := range classList {
				taskChan <- class
			}
			close(taskChan)
		}()

		// 启动 worker goroutines
		for i := 0; i < concurrentLimit; i++ {
			wg.Add(1)
			go r.worker(taskChan, &wg, i+1)
		}

		// 收集结果
		wg.Wait()
		// 检查 resultChan 的结果
		result := r.Success.Load()
		if !result {
			r.formatLog("没课可选了，正在重试...")
			if r.config.MinCredit-1 < 1 {
				r.config.MinCredit = 1
			} else {
				r.config.MinCredit = r.config.MinCredit - 1
			}
			r.formatLog(fmt.Sprintf("降低学分要求为[%d]，正在重试...", r.config.MinCredit))
		} else {
			fmt.Println("退出")
			r.StopRob()
			r.stopEvent()
			break
		}
	}
}

func (r *Robber) doSelect(classInfoList []map[string]interface{}, item map[string]interface{}, thread int) int {
	// 检查 context 是否已取消
	select {
	case <-r.Ctx.Done():
		r.stopEvent()
		return -1
	default:
	}

	if r.Success.Load() {
		r.formatLog(fmt.Sprintf("已经抢过了-线程[%d]", thread))
		return 1
	}

	var res map[string]interface{}
	var err error
	for _, classInfo := range classInfoList {
		jxbrl := getString(classInfo["jxbrl"])
		yxzrs := getString(item["yxzrs"])
		//jxbmc := getString(item["jxbmc"])
		kcmc := getString(item["kcmc"])
		teacherNameStr := getString(classInfo["jsxx"])
		teacherName := ""
		tNArr := strings.Split(teacherNameStr, "/")
		if len(tNArr) < 2 {
			teacherName = teacherNameStr
		} else {
			teacherName = tNArr[1]
		}

		if !r.isCourseValid(item) {
			kch := getString(item["kch"])
			r.formatLog(fmt.Sprintf("%s-%s-不符合课程号要求[课程号：%s,%s]-线程[%d]", kcmc, teacherName, kch, strings.Join(r.config.CourseNumList, "|"), thread))
			continue
		}

		// 通过课程号校验 但学分不符
		if r.isCourseValid(item) && !r.isMinCreditValid(item) {
			xf := getString(item["xf"])
			r.formatLog(fmt.Sprintf("%s-%s-不符合学分要求[学分：%s/%d]-线程[%d]", kcmc, teacherName, xf, r.config.MinCredit, thread))
			continue
		}

		if toInt(jxbrl) <= toInt(yxzrs) {
			r.formatLog(fmt.Sprintf("%s-%s人满了[%s/%s]-线程[%d]", kcmc, teacherName, yxzrs, jxbrl, thread))
			continue
		}

		// 看看是不是指定老师
		if r.config.CourseType != 0 {
			if !strings.Contains(teacherName, r.config.TeacherName) {
				r.formatLog(fmt.Sprintf("当前课程:%s-老师:%s-指定:%s不符合，跳过", kcmc, teacherName, r.config.TeacherName))
				continue
			}
		}
		//fmt.Printf("%+v\n", item)
		res, err = r.jwxt.DoSelect(classInfo, item)
		if err != nil {
			continue
		}

		// 如果是-1 那么立即再抢一次
		flag := getInt(res["flag"])
		if flag == -1 {
			res, err = r.jwxt.DoSelect(classInfo, item)
		}
		r.formatLog(fmt.Sprintf("抢课[%s]结果：%d", kcmc, flag))
		if flag == 1 {
			// 抢到了
			r.formatLog(fmt.Sprintf("抢到了-%s-线程[%d]", kcmc, thread))
			r.isBug.Store(false)
			r.Success.Store(true)
			return 1
		} else {
			msg := getString(res["msg"])
			if strings.Contains(msg, "超过通识选修课本学期最高选课门次限制，不可选！") {
				r.formatLog(fmt.Sprintf("已经抢过了-线程[%d]", thread))
				r.isBug.Store(false)
				r.Success.Store(true)
				return 1
			} else if strings.Contains(msg, "未完成评价，不可选课！") {
				r.formatLog(fmt.Sprintf("出现系统bug，再次尝试中-线程[%d]", thread))
				r.isBug.Store(true)
			} else if strings.Contains(msg, "非法") {
				r.formatLog(fmt.Sprintf("非法操作,正在重试-线程[%d]", thread))
				return r.doSelect(classInfoList, item, thread)
			}
		}

		// 延迟
		time.Sleep(3 * time.Millisecond)
	}
	if len(res) == 0 {
		return -1
	}
	return 0
}

func (r *Robber) formatLog(log string) {
	msg := fmt.Sprintf("[%s]-%s-%s", getNow(), r.config.Account, log)
	r.Logger.AppendLog(msg)
	fmt.Println(msg)
}

func (r *Robber) stopEvent() {
	r.formatLog("抢课已停止")
	r.loginRes.Store(false)
	go component.StopButton(r.ui)
}

func (r *Robber) login() error {
	err := r.jwxt.login()
	return err
}

func (r *Robber) isCourseValid(item map[string]interface{}) bool {
	if len(r.config.CourseNumList) == 0 {
		return true
	}
	if r.config.CourseType != 0 {
		return true
	}
	kch := getString(item["kch"])
	if len(r.config.CourseNumList) > 0 && indexOf(kch, r.config.CourseNumList) != -1 {
		return true
	}
	return false
}

func (r *Robber) isMinCreditValid(item map[string]interface{}) bool {
	xf := toFloat(getString(item["xf"]))
	if xf >= float64(r.config.MinCredit) {
		return true
	}
	return false
}
