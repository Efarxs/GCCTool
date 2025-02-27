package model

import (
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

type UIComponents struct {
	AccountEntry       *widget.Entry
	PasswordEntry      *widget.Entry
	HourEntry          *widget.Entry
	MinuteEntry        *widget.Entry
	ComboBox           *widget.Select
	CheckBoxes         [9]*widget.Check
	CourseNameEntry    *widget.Entry
	TeacherEntry       *widget.Entry
	CourseNumListEntry *widget.Entry
	RadioButtonGroup   *widget.RadioGroup
	MinCreditEntry     *widget.Entry
	StartButton        *widget.Button
	StopButton         *widget.Button
	LogLabel           *widget.Label
	LogScroll          *container.Scroll
	ThreadNumEntry     *widget.Entry
	AHeadMinuteEntry   *widget.Entry
	AgentEntry         *widget.Entry
	CopyButton         *widget.Button
}
