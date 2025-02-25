package rob

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"jwxt/model"
	"jwxt/pkg/errs"
	"jwxt/pkg/logger"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type JwxtRob struct {
	host       string
	apiURL     string
	headerMap  map[string]string
	robConfig  model.Config
	cookieJar  *cookiejar.Jar
	httpClient *http.Client
	httpCode   int
	postArr1   map[string]string
	postArr2   map[string]string

	logger   *logger.Logger
	AgentUrl string
}

func NewJwxtRob(robConfig model.Config, logger *logger.Logger) *JwxtRob {
	jwxt := &JwxtRob{
		host:      "4nx8821287.goho.co",
		robConfig: robConfig,
		headerMap: make(map[string]string),
		postArr1:  make(map[string]string),
		postArr2:  make(map[string]string),
		logger:    logger,
		AgentUrl:  robConfig.AgentUrl,
	}
	jwxt.apiURL = "http://" + jwxt.host
	if robConfig.URL != "" {
		jwxt.apiURL = "http://" + robConfig.URL
	}
	jwxt.initHeader()

	return jwxt
}

func (j *JwxtRob) initHeader() {
	j.cookieJar, _ = cookiejar.New(nil)
	j.httpClient = &http.Client{
		Jar: j.cookieJar,
	}

	j.headerMap["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36"
	j.headerMap["Host"] = j.host
	j.headerMap["Content-Type"] = "application/x-www-form-urlencoded"
	j.headerMap["Accept-Language"] = "zh-cn"
	j.headerMap["X-Target-URL"] = j.apiURL
	j.headerMap["X-JW-Account"] = j.robConfig.Account
	j.headerMap["X-JW-Password"] = j.robConfig.Password
}

func (j *JwxtRob) curl(url string, postData string) (string, error) {
	var req *http.Request
	var err error

	if j.AgentUrl != "" {
		//url = fmt.Sprintf("%s/%s", j.AgentUrl, strings.Replace(url, "http://", "", 1))
		url = strings.Replace(url, j.apiURL, j.AgentUrl, 1)
	}
	if postData != "" {
		req, err = http.NewRequest("POST", url, strings.NewReader(postData))
	} else {
		req, err = http.NewRequest("GET", url, nil)
	}

	if err != nil {
		return "", err
	}

	for key, value := range j.headerMap {
		req.Header.Set(key, value)
	}

	resp, err := j.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func(Body io.ReadCloser) {
		err = Body.Close()
		if err != nil {

		}
	}(resp.Body)

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	j.httpCode = resp.StatusCode
	return string(body), nil
}

// login 登录
func (j *JwxtRob) login() error {
	urlLogin := j.apiURL + "/xtgl/login_slogin.html"
	res, err := j.curl(urlLogin, "")
	if err != nil {
		return errs.ErrServerFail
	}

	// 提取 CSRF Token
	csrfToken := ""
	pattern := `id="csrftoken".*?value="(.*?)"`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(res)
	if len(matches) > 1 {
		csrfToken = matches[1]
	}

	// 获取公钥
	urlPublicKey := j.apiURL + "/xtgl/login_getPublicKey.html"
	res, err = j.curl(urlPublicKey, "")
	if err != nil {
		return errs.ErrServerFail
	}

	var keyData map[string]string
	if err = json.Unmarshal([]byte(res), &keyData); err != nil {
		return errs.ErrServerFail
	}

	modulus, err := base64.StdEncoding.DecodeString(keyData["modulus"])
	if err != nil {
		return errs.ErrServerFail
	}

	exponent, err := base64.StdEncoding.DecodeString(keyData["exponent"])
	if err != nil {
		return errs.ErrServerFail
	}

	// 构建 RSA 公钥
	mb := new(big.Int).SetBytes(modulus)
	eb := new(big.Int).SetBytes(exponent)
	pubKey := &rsa.PublicKey{
		N: mb,
		E: int(eb.Int64()),
	}

	// 加密密码
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, pubKey, []byte(j.robConfig.Password))
	if err != nil {
		return errs.ErrServerFail
	}
	enPassword := base64.StdEncoding.EncodeToString(encrypted)

	// 二次登录
	postData := "csrftoken=" + csrfToken + "&language=zh_CN&yhm=" + j.robConfig.Account + "&mm=" + url.QueryEscape(enPassword)
	res, err = j.curl(urlLogin, postData)
	if err != nil {
		return errs.ErrServerFail
	}

	// 检查登录结果
	if j.httpCode == 201 {
		return errs.ErrLoginFail
	}
	if strings.Contains(res, "用户名或密码") {
		return errs.ErrPassword
	}
	//if j.httpCode == 302 {
	//	return nil
	//}
	//
	//return errors.New("登录失败")

	return nil
}

// initPostArr1 初始化 postArr1 数据
func (j *JwxtRob) initPostArr1() error {
	u := j.apiURL + "/xsxk/zzxkyzb_cxZzxkYzbIndex.html?gnmkdm=N253512&su=" + j.robConfig.Account
	res, err := j.curl(u, "")

	if err != nil {
		return errs.ErrServerFail
	}

	if strings.Contains(res, "用户登录") {
		return errs.ErrLoginExpired
	}

	//m := mock.NewMock()
	//res = m.GetClassList()
	if strings.Contains(res, "当前不属于选课阶段") {
		return errs.ErrRobTime
	}

	getPostDataMap(res, j.postArr1)
	return nil
}

// initPostArr2 初始化 postArr2 数据
func (j *JwxtRob) initPostArr2() error {
	u := j.apiURL + "/xsxk/zzxkyzb_cxZzxkYzbDisplay.html?gnmkdm=N253512&su=" + j.robConfig.Account
	postData := "xkkz_id=" + j.postArr1["firstXkkzId"] + "&xszxzt=1&kspage=0&jspage=0"
	res, err := j.curl(u, postData)
	//m := mock.NewMock()
	//res = m.Display()
	if err != nil {
		return errors.New("网络异常")
	}
	if strings.Contains(res, "用户登录") {
		return errors.New("被抢登录了")
	}

	getPostDataMap(res, j.postArr2)
	return nil
}

// postDataCommon 将 postArr1 和 postArr2 中的数据合并到 postDataMap 中
func (j *JwxtRob) postDataCommon(postDataMap map[string]string) {
	// 从 postArr2 中提取数据
	postDataMap["sfkknj"] = j.postArr2["sfkknj"]
	postDataMap["sfkkzy"] = j.postArr2["sfkkzy"]
	postDataMap["kzybkxy"] = j.postArr2["kzybkxy"]
	postDataMap["sfznkx"] = j.postArr2["sfznkx"]
	postDataMap["zdkxms"] = j.postArr2["zdkxms"]
	postDataMap["sfkxq"] = j.postArr2["sfkxq"]
	postDataMap["sfkcfx"] = j.postArr2["sfkcfx"]
	postDataMap["kkbk"] = j.postArr2["kkbk"]
	postDataMap["kkbkdj"] = j.postArr2["kkbkdj"]

	// 从 postArr1 中提取数据
	postDataMap["zyfx_id"] = j.postArr1["zyfx_id"]
	postDataMap["njdm_id"] = j.postArr1["njdm_id"]
	postDataMap["bh_id"] = j.postArr1["bh_id"]
	postDataMap["xbm"] = j.postArr1["xbm"]
	postDataMap["xslbdm"] = j.postArr1["xslbdm"]
	postDataMap["mzm"] = j.postArr1["mzm"]
	postDataMap["xz"] = j.postArr1["xz"]

	// 从 postArr2 中提取数据
	postDataMap["rwlx"] = j.postArr2["rwlx"]
	postDataMap["xkly"] = j.postArr2["xkly"]
	postDataMap["bklx_id"] = j.postArr2["bklx_id"]
	postDataMap["sfkkjyxdxnxq"] = j.postArr2["sfkkjyxdxnxq"]
	postDataMap["xqh_id"] = j.postArr1["xqh_id"]
}

func (j *JwxtRob) GetClassList() ([]map[string]interface{}, error) {

	err := j.initPostArr1()
	if err != nil {
		return nil, errs.ErrDataInit
	}
	err = j.initPostArr2()
	if err != nil {
		return nil, errs.ErrDataInit
	}
	// 查询课程
	u := j.apiURL + "/xsxk/zzxkyzb_cxZzxkYzbPartDisplay.html?gnmkdm=N253512&su=" + j.robConfig.Account

	postDataMap := make(map[string]string)
	if j.robConfig.CourseType == 0 {
		// 如果是 0
		postDataMap["kkbm_id_list[0]"] = "70" // 教务处，这里写死了网课
	} else if j.robConfig.CourseType == 1 {
		// 体育课
		//postDataMap["filter_list[0]"] = strconv.Itoa(j.robConfig.CourseType)
		postDataMap["filter_list[0]"] = j.robConfig.CourseName
	}

	// 添加其他参数
	postDataMap["gnjkxdnj"] = j.postArr1["gnjkxdnj"]
	postDataMap["bjgkczxbbjwcx"] = j.postArr1["bjgkczxbbjwcx"]
	postDataMap["xqh_id"] = j.postArr1["xqh_id"]
	postDataMap["sfkkjyxdxnxq"] = j.postArr1["sfkkjyxdxnxq"]
	postDataMap["bklx_id"] = j.postArr1["bklx_id"]
	postDataMap["xkly"] = j.postArr1["xkly"]
	postDataMap["rwlx"] = j.postArr1["rwlx"]
	postDataMap["jg_id"] = j.postArr1["jg_id_1"]
	postDataMap["njdm_id_1"] = j.postArr1["njdm_id_1"]
	postDataMap["zyh_id_1"] = j.postArr1["zyh_id_1"]
	postDataMap["zyh_id"] = j.postArr1["zyh_id_1"]
	postDataMap["ccdm"] = j.postArr1["ccdm"]
	postDataMap["xsbj"] = j.postArr1["xsbj"]
	postDataMap["sfkgbcx"] = j.postArr2["sfkgbcx"]
	postDataMap["sfrxtgkcxd"] = j.postArr2["sfrxtgkcxd"]
	postDataMap["tykczgxdcs"] = j.postArr2["tykczgxdcs"]
	postDataMap["xkxnm"] = j.postArr1["xkxnm"]
	postDataMap["xkxqm"] = j.postArr1["xkxqm"]
	postDataMap["kklxdm"] = j.postArr1["firstKklxdm"]
	postDataMap["bbhzxjxb"] = j.postArr2["bbhzxjxb"]
	postDataMap["rlkz"] = j.postArr2["rlkz"]
	postDataMap["xkzgbj"] = j.postArr2["xkzgbj"]
	postDataMap["kspage"] = "1"
	postDataMap["jspage"] = "1480"
	postDataMap["jxbzb"] = ""

	// 只有在没有指定课程 ID 的时候才会根据课程类别搜索
	if len(j.robConfig.CourseNumList) == 0 {
		for i, courseType := range j.robConfig.CourseNumList {
			postDataMap["kcgs_list["+strconv.Itoa(i)+"]"] = courseType
		}
	}

	// 设置请求头
	j.headerMap["Accept"] = "application/json, text/javascript, */*; q=0.01"
	j.headerMap["Accept-Language"] = "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7,zh-TW;q=0.6"
	j.headerMap["Connection"] = "keep-alive"
	j.headerMap["Content-Type"] = "application/x-www-form-urlencoded;charset=UTF-8"
	j.headerMap["X-Jw.requested-With"] = "XMLHttpJw.request"

	var postData = map2String(postDataMap)

	// 发送请求
	res, err := j.curl(u, postData)
	//m := mock.NewMock()
	//res = m.GetPart()
	if err != nil {
		return nil, errs.ErrServerFail
	}

	// 解析响应
	var result map[string]interface{}
	if err = json.Unmarshal([]byte(res), &result); err != nil {
		return nil, errs.ErrServerFail
	}

	// 检查是否包含课程列表
	if tmpList, ok := result["tmpList"].([]interface{}); ok && len(tmpList) > 0 {
		j.formatLog("初始化完毕，成功获取可选课表")
		courseList := make([]map[string]interface{}, len(tmpList))
		for i, item := range tmpList {
			courseList[i] = item.(map[string]interface{})
		}
		return courseList, nil
	}

	j.formatLog("可选课表初始化失败...")
	return nil, errs.ErrInitClassInfo
}

// doSelect 选课
func (j *JwxtRob) DoSelect(kcArr, oldKcArr map[string]interface{}) (map[string]interface{}, error) {
	u := j.apiURL + "/xsxk/zzxkyzbjk_xkBcZyZzxkYzb.html?gnmkdm=N253512&su=" + j.robConfig.Account

	postDataMap := make(map[string]string)
	postDataMap["jxb_ids"] = getString(kcArr["do_jxb_id"])
	postDataMap["kch_id"] = getString(oldKcArr["kch_id"])
	postDataMap["kcmc"] = "(" + getString(oldKcArr["kch"]) + ")" + getString(oldKcArr["kcmc"]) + "-" + getString(oldKcArr["xf"]) + "学分"
	postDataMap["rwlx"] = j.postArr2["rwlx"]
	postDataMap["rlkz"] = j.postArr2["rlkz"]
	postDataMap["rlzlkz"] = j.postArr2["rlzlkz"]
	postDataMap["sxbj"] = j.postArr2["sxbj"]
	postDataMap["xxkbj"] = getString(oldKcArr["xxkbj"])
	postDataMap["qz"] = "0"
	postDataMap["cxbj"] = getString(oldKcArr["cxbj"])
	postDataMap["xkkz_id"] = j.postArr1["xkkz_id"]
	postDataMap["njdm_id"] = j.postArr1["njdm_id"]
	postDataMap["zyh_id"] = j.postArr1["zyh_id"]
	postDataMap["kklxdm"] = j.postArr1["firstKklxdm"]
	postDataMap["xklc"] = j.postArr2["xklc"]
	postDataMap["xkxnm"] = j.postArr1["xkxnm"]
	postDataMap["xkxqm"] = j.postArr1["xkxqm"]

	j.formatLog("正在抢课:" + postDataMap["kcmc"])
	res, err := j.curl(u, map2String(postDataMap))
	if err != nil {
		return nil, errs.ErrServerFail
	}

	var result map[string]interface{}
	if err = json.Unmarshal([]byte(res), &result); err != nil {
		return nil, errs.ErrServerFail
	}

	return result, nil
}

// getClassInfo 获取课程信息
func (j *JwxtRob) GetClassInfo(kcArr map[string]interface{}) ([]map[string]interface{}, error) {
	u := j.apiURL + "/xsxk/zzxkyzbjk_cxJxbWithKchZzxkYzb.html?gnmkdm=N253512&su=" + j.robConfig.Account

	postDataMap := make(map[string]string)
	j.postDataCommon(postDataMap)
	postDataMap["jg_id"] = j.postArr2["jg_id"]
	postDataMap["zyh_id"] = j.postArr1["zyh_id"]
	postDataMap["bbhzxjxb"] = j.postArr2["bbhzxjxb"]
	postDataMap["ccdm"] = j.postArr1["ccdm"]
	postDataMap["xsbj"] = j.postArr1["xsbj"]
	postDataMap["xkxnm"] = j.postArr1["xkxnm"]
	postDataMap["xkxqm"] = j.postArr1["xkxqm"]
	postDataMap["xkxskcgskg"] = "1"
	postDataMap["rlkz"] = j.postArr1["rlkz"]
	postDataMap["kklxdm"] = j.postArr1["firstKklxdm"]
	postDataMap["kch_id"] = getString(kcArr["kch_id"])
	postDataMap["jxbzcxskg"] = "0"
	postDataMap["xkkz_id"] = j.postArr1["firstXkkzId"]
	postDataMap["cxbj"] = getString(kcArr["cxbj"])
	postDataMap["fxbj"] = getString(kcArr["fxbj"])

	res, err := j.curl(u, map2String(postDataMap))
	if err != nil {
		return nil, errs.ErrServerFail
	}
	//m := mock.NewMock()
	//res = m.GetKch()

	var result []map[string]interface{}
	if err = json.Unmarshal([]byte(res), &result); err != nil {
		return nil, errs.ErrServerFail
	}

	if len(result) > 0 {
		//fmt.Println(j.robConfig.Account + "-读取课程数据【" + kcArr["kcmc"] + "】-" + kcArr["jxbmc"] + "信息成功...")
		return result, nil
	}

	j.formatLog("读取课程失败...")
	return nil, errs.ErrClassInfo
}

// IsLogin 判断是否登录
func (j *JwxtRob) IsLogin() (bool, error) {
	u := j.apiURL + "/xtgl/index_cxGxDlztxx.html?dlztxxtj_id="
	res, err := j.curl(u, "")
	if err != nil {
		return false, errs.ErrServerFail
	}

	if res == "" {
		// 已经登录了
		j.formatLog("维持登录状态：正常")
		return true, nil
	}

	// 已经过期了
	j.formatLog("维持登录状态：已经失效，正在重新登录")
	return false, errs.ErrLoginExpired
}

func (j *JwxtRob) formatLog(log string) {
	msg := fmt.Sprintf("%s-%s-%s", j.robConfig.Account, getNow(), log)
	j.logger.AppendLog(msg)
	fmt.Println(msg)
}
