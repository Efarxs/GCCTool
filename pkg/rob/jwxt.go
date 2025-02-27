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

func (j *JwxtRob) getInputValue(key string) string {
	v1, ok1 := j.postArr1[key]
	v2, ok2 := j.postArr2[key]
	if ok1 {
		return v1
	}
	if ok2 {
		return v2
	}
	return ""
}

// postDataCommon 将 postArr1 和 postArr2 中的数据合并到 postDataMap 中
func (j *JwxtRob) postDataCommon(postDataMap map[string]string) {
	// 从 postArr2 中提取数据
	postDataMap["sfkknj"] = j.getInputValue("sfkknj")
	postDataMap["sfkkzy"] = j.getInputValue("sfkkzy")
	postDataMap["kzybkxy"] = j.getInputValue("kzybkxy")
	postDataMap["sfznkx"] = j.getInputValue("sfznkx")
	postDataMap["zdkxms"] = j.getInputValue("zdkxms")
	postDataMap["sfkxq"] = j.getInputValue("sfkxq")
	postDataMap["sfkcfx"] = j.getInputValue("sfkcfx")
	postDataMap["kkbk"] = j.getInputValue("kkbk")
	postDataMap["kkbkdj"] = j.getInputValue("kkbkdj")

	// 从 postArr1 中提取数据
	postDataMap["zyfx_id"] = j.getInputValue("zyfx_id")
	postDataMap["njdm_id"] = j.getInputValue("njdm_id")
	postDataMap["bh_id"] = j.getInputValue("bh_id")
	postDataMap["xbm"] = j.getInputValue("xbm")
	postDataMap["xslbdm"] = j.getInputValue("xslbdm")
	postDataMap["mzm"] = j.getInputValue("mzm")
	postDataMap["xz"] = j.getInputValue("xz")

	// 从 postArr2 中提取数据
	postDataMap["rwlx"] = j.getInputValue("rwlx")
	postDataMap["xkly"] = j.getInputValue("xkly")
	postDataMap["bklx_id"] = j.getInputValue("bklx_id")
	postDataMap["sfkkjyxdxnxq"] = j.getInputValue("sfkkjyxdxnxq")
	postDataMap["xqh_id"] = j.getInputValue("xqh_id")

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

	// kcgs_list%5B0%5D=1&kcgs_list%5B1%5D=2&kcgs_list%5B2%5D=3&kcgs_list%5B3%5D=4&kcgs_list%5B4%5D=5&rwlx=2&xklc=2&xkly=0&bklx_id=0&sfkkjyxdxnxq=0&kzkcgs=0&xqh_id=1&jg_id=12&njdm_id_1=2021&zyh_id_1=1201&gnjkxdnj=0&zyh_id=1201&zyfx_id=wfx&njdm_id=2021&bh_id=12012102&bjgkczxbbjwcx=0&xbm=1&xslbdm=421&mzm=01&xz=4&ccdm=3&xsbj=0&sfkknj=0&sfkkzy=0&kzybkxy=0&sfznkx=0&zdkxms=0&sfkxq=0&sfkcfx=0&kkbk=0&kkbkdj=0&sfkgbcx=0&sfrxtgkcxd=0&tykczgxdcs=0&xkxnm=2024&xkxqm=12&kklxdm=10&bbhzxjxb=0&xkkz_id=2F05D8896BE84A65E065000000000001&rlkz=0&xkzgbj=0&kspage=1&jspage=10&jxbzb=
	// 添加其他参数
	postDataMap["rwlx"] = j.getInputValue("rwlx")
	postDataMap["xklc"] = j.getInputValue("xklc")
	postDataMap["xkly"] = j.getInputValue("xkly")
	postDataMap["bklx_id"] = j.getInputValue("bklx_id")
	postDataMap["sfkkjyxdxnxq"] = j.getInputValue("sfkkjyxdxnxq")
	postDataMap["kzkcgs"] = j.getInputValue("kzkcgs")
	postDataMap["xqh_id"] = j.getInputValue("xqh_id")
	postDataMap["jg_id"] = j.getInputValue("jg_id_1")
	postDataMap["njdm_id_1"] = j.getInputValue("njdm_id_1")
	postDataMap["zyh_id_1"] = j.getInputValue("zyh_id_1")
	postDataMap["gnjkxdnj"] = j.getInputValue("gnjkxdnj")
	postDataMap["zyh_id"] = j.getInputValue("zyh_id")
	postDataMap["zyfx_id"] = j.getInputValue("zyfx_id")
	postDataMap["njdm_id"] = j.getInputValue("njdm_id")
	postDataMap["bh_id"] = j.getInputValue("bh_id")
	postDataMap["bjgkczxbbjwcx"] = j.getInputValue("bjgkczxbbjwcx")
	postDataMap["xbm"] = j.getInputValue("xbm")
	postDataMap["xslbdm"] = j.getInputValue("xslbdm")
	postDataMap["mzm"] = j.getInputValue("mzm")
	postDataMap["ccdm"] = j.getInputValue("ccdm")
	postDataMap["xz"] = j.getInputValue("xz")
	postDataMap["xsbj"] = j.getInputValue("xsbj")
	postDataMap["sfkknj"] = j.getInputValue("sfkknj")
	postDataMap["sfkkzy"] = j.getInputValue("sfkkzy")
	postDataMap["kzybkxy"] = j.getInputValue("kzybkxy")
	postDataMap["sfznkx"] = j.getInputValue("sfznkx")
	postDataMap["zdkxms"] = j.getInputValue("zdkxms")
	postDataMap["sfkxq"] = j.getInputValue("sfkxq")
	postDataMap["sfkcfx"] = j.getInputValue("sfkcfx")
	postDataMap["kkbk"] = j.getInputValue("kkbk")
	postDataMap["kkbkdj"] = j.getInputValue("kkbkdj")
	postDataMap["sfkgbcx"] = j.getInputValue("sfkgbcx")
	postDataMap["sfrxtgkcxd"] = j.getInputValue("sfrxtgkcxd")
	postDataMap["tykczgxdcs"] = j.getInputValue("tykczgxdcs")
	postDataMap["xkxnm"] = j.getInputValue("xkxnm")
	postDataMap["xkxqm"] = j.getInputValue("xkxqm")
	postDataMap["kklxdm"] = j.getInputValue("firstKklxdm")
	postDataMap["bbhzxjxb"] = j.getInputValue("bbhzxjxb")
	postDataMap["xkkz_id"] = j.getInputValue("firstXkkzId")
	postDataMap["rlkz"] = j.getInputValue("rlkz")
	postDataMap["xkzgbj"] = j.getInputValue("xkzgbj")
	postDataMap["kspage"] = "1"
	postDataMap["jspage"] = "1480"
	postDataMap["jxbzb"] = ""
	//postDataMap["yl_list[0]"] = "1"

	// 只有在没有指定课程 ID 的时候才会根据课程类别搜索
	if len(j.robConfig.CourseNumList) == 0 {
		for i, courseType := range j.robConfig.CategoryList {
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

	fmt.Println(postDataMap)
	fmt.Println(postData)

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

	fmt.Println(result)

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
	postDataMap["rwlx"] = j.getInputValue("rwlx")
	postDataMap["rlkz"] = j.getInputValue("rlkz")
	postDataMap["rlzlkz"] = j.getInputValue("rlzlkz")
	postDataMap["sxbj"] = j.getInputValue("sxbj")
	postDataMap["xxkbj"] = getString(oldKcArr["xxkbj"])
	postDataMap["qz"] = "0"
	postDataMap["cxbj"] = getString(oldKcArr["cxbj"])
	postDataMap["xkkz_id"] = j.getInputValue("xkkz_id")
	postDataMap["njdm_id"] = j.getInputValue("njdm_id")
	postDataMap["zyh_id"] = j.getInputValue("zyh_id")
	postDataMap["kklxdm"] = j.getInputValue("firstKklxdm")
	postDataMap["xklc"] = j.getInputValue("xklc")
	postDataMap["xkxnm"] = j.getInputValue("xkxnm")
	postDataMap["xkxqm"] = j.getInputValue("xkxqm")

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
	postDataMap["jg_id"] = j.getInputValue("jg_id_1")
	postDataMap["zyh_id"] = j.getInputValue("zyh_id")
	postDataMap["bbhzxjxb"] = j.getInputValue("bbhzxjxb")
	postDataMap["ccdm"] = j.getInputValue("ccdm")
	postDataMap["xsbj"] = j.getInputValue("xsbj")
	postDataMap["xkxnm"] = j.getInputValue("xkxnm")
	postDataMap["xkxqm"] = j.getInputValue("xkxqm")
	postDataMap["xkxskcgskg"] = "1"
	postDataMap["rlkz"] = j.getInputValue("rlkz")
	postDataMap["kklxdm"] = j.getInputValue("firstKklxdm")
	postDataMap["kch_id"] = getString(kcArr["kch_id"])
	postDataMap["jxbzcxskg"] = "0"
	postDataMap["xkkz_id"] = j.getInputValue("firstXkkzId")
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
