package model

type Config struct {
	URL           string
	Account       string
	Password      string
	Time          string
	Hour          string
	Minute        string
	MinCredit     int
	CourseType    int
	CourseName    string
	TeacherName   string
	CourseNumList []string

	ThreadNum   int
	AHeadMinute int
	AgentUrl    string
}
