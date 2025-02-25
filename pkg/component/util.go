package component

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
	"jwxt/model"
)

func StartButton(ui *model.UIComponents) {
	ui.StartButton.Disable()
	ui.StopButton.Enable()
	enableComponents(false, ui.AccountEntry, ui.PasswordEntry, ui.HourEntry, ui.MinuteEntry, ui.MinCreditEntry, ui.CourseNameEntry, ui.TeacherEntry, ui.CourseNumListEntry, ui.ComboBox, ui.ThreadNumEntry)
	for _, checkBox := range ui.CheckBoxes {
		enableComponents(false, checkBox)
	}
	enableComponents(false, ui.RadioButtonGroup)
}

func StopButton(ui *model.UIComponents) {
	ui.StopButton.Disable()
	ui.StartButton.Enable()
	enableComponents(true, ui.AccountEntry, ui.PasswordEntry, ui.HourEntry, ui.MinuteEntry, ui.MinCreditEntry, ui.CourseNameEntry, ui.TeacherEntry, ui.CourseNumListEntry, ui.ComboBox, ui.ThreadNumEntry)
	for _, checkBox := range ui.CheckBoxes {
		enableComponents(true, checkBox)
	}
	enableComponents(true, ui.RadioButtonGroup)
}

func enableComponents(enable bool, components ...fyne.CanvasObject) {
	for _, component := range components {
		switch c := component.(type) {
		case *widget.Entry:
			if enable {
				c.Enable()
			} else {
				c.Disable()
			}
		case *widget.Select:
			if enable {
				c.Enable()
			} else {
				c.Disable()
			}
		case *widget.Check:
			if enable {
				c.Enable()
			} else {
				c.Disable()
			}
		case *widget.RadioGroup:
			if enable {
				c.Enable()
			} else {
				c.Disable()
			}
		}
	}
}
