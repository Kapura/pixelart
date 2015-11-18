package main

import (
	"fmt"
	"github.com/google/gxui"
	"github.com/google/gxui/drivers/gl"
	"github.com/google/gxui/math"
	"github.com/google/gxui/themes/light"
	"image"
	//"image/draw"
	"runtime"
	"strconv"
	"strings"
)

var (
	FieldNames []string
	ProgPic gxui.Image
	Driver gxui.Driver
)

type dataField struct {
	label gxui.Label
	input gxui.TextBox
	err gxui.Label
	layout gxui.LinearLayout
}

func (df *dataField) Get() string {
	return df.input.Text()
}

func (df *dataField) Put(s string) {
	df.input.SetText(s)
}

func (df *dataField) SetLabel(label string) {
	df.label.SetText(label)
}

func (df *dataField) SetError(e string) {
	df.err.SetText(e)
}

func makeDataField(theme gxui.Theme, label string) *dataField {
	df := new(dataField)
	df.label = theme.CreateLabel()
	df.label.SetText(label)
	df.input = theme.CreateTextBox()
	df.err = theme.CreateLabel()
	df.err.SetColor(gxui.Red)
	df.layout = theme.CreateLinearLayout()
	df.layout.SetDirection(gxui.LeftToRight)
	df.layout.AddChild(df.label)
	df.layout.AddChild(df.input)
	df.layout.AddChild(df.err)

	return df
}


func appMain(driver gxui.Driver) {
	data := make(map[string]*dataField)
	output := make(map[string]gxui.Label)

	theme := light.CreateTheme(driver)

	window := theme.CreateWindow(750, 512, "art")
	window.OnClose(driver.Terminate)

	h_layout := theme.CreateLinearLayout()
	h_layout.SetDirection(gxui.LeftToRight)
	window.AddChild(h_layout)

	v_layout := theme.CreateLinearLayout()
	v_layout.SetDirection(gxui.TopToBottom)
	v_layout.SetHorizontalAlignment(gxui.AlignRight)
	h_layout.AddChild(v_layout)

	ProgPic = theme.CreateImage()
	ProgPic.SetExplicitSize(math.Size{512, 512})
	h_layout.AddChild(ProgPic)

	for s := range FieldNames {
		df := makeDataField(theme, FieldNames[s])
		data[FieldNames[s]] = df
		v_layout.AddChild(df.layout)
	}
	initialise(data)

	run_button := theme.CreateButton()
	run_button.SetText("Run")
	run_button.OnClick(func(gxui.MouseEvent) { onRun(data, output) })

	label_3 := theme.CreateLabel()
	output["L3"] = label_3

	v_layout.AddChild(run_button)

	Driver = driver
}

func onRun(data map[string]*dataField, output map[string]gxui.Label) {
	valid, args := validate(data)
	if valid {
		go Generate(args)
	}

}

func initialise(data map[string]*dataField) {
	data["chan size"].Put("8")
	data["blur"].Put("1")
	data["cpus"].Put(fmt.Sprint(runtime.NumCPU()))
	data["update freq"].Put("10")
	data["colour basis"].Put("rgb")
	data["echospacing"].Put("0")
	data["flip draw"].Put("false")
	data["intermediate steps"].Put("false")
	data["seed colour"].Put("0x000000")
	data["start X"].Put("0")
	data["start Y"].Put("0")
	data["tag"].Put("art")
	data["width"].Put("4096")
	data["height"].Put("4096")
}

func validate(data map[string]*dataField) (valid bool, args GenerateArgs) {
	valid = true
	e_msg := ""

	cpu, err := strconv.Atoi(data["cpus"].Get())
	if err != nil || cpu < 1 || cpu > runtime.NumCPU() {
		valid = false
		e_msg = fmt.Sprintf("CPUs must be between 1 and %d", runtime.NumCPU())
		data["cpus"].SetError(e_msg)
	} else {
		data["cpus"].SetError("")
		args.cpus = cpu
	}

	ch, err := strconv.Atoi(data["chan size"].Get())
	if err != nil || ch < 8 {
		valid = false
		e_msg = "Channel size should be a number greater than 8"
		data["chan size"].SetError(e_msg)
	} else {
		data["chan size"].SetError("")
		args.chan_size = int32(ch)
	}

	bl, err := strconv.Atoi(data["blur"].Get())
	if err != nil || bl < 1 || bl > 50 {
		valid = false
		e_msg = "Blur should be a number between 1 and 50"
		data["blur"].SetError(e_msg)
	} else {
		data["blur"].SetError("")
		args.blur = int32(bl)
	}

	uf, err := strconv.Atoi(data["update freq"].Get())
	if err != nil || uf < 1 || uf > 4096 {
		valid = false
		e_msg = "Update frequency should be between 1 and 4096"
		data["update freq"].SetError(e_msg)
	} else {
		data["update freq"].SetError("")
		args.update_freq = int32(uf)
	}

	cb := strings.ToLower(data["colour basis"].Get())
	if cb != "rgb" && cb != "rbg" && cb != "brg" && cb != "bgr" && cb != "grb" && cb != "gbr" {
		valid = false
		e_msg = "Colour Basis should be a combination of r, g, and b (ex. 'rgb')"
		data["colour basis"].SetError(e_msg)
	} else {
		data["colour basis"].SetError("")
		switch cb {
		case "rgb":
			args.colour_basis = RGB
		case "rbg":
			args.colour_basis = RBG
		case "gbr":
			args.colour_basis = GBR
		case "grb":
			args.colour_basis = GRB
		case "bgr":
			args.colour_basis = BGR
		case "brg":
			args.colour_basis = BRG
		}
	}

	es, err := strconv.ParseFloat(data["echospacing"].Get(), 64)
	if err != nil || es < 0 || es > 1 {
		valid = false
		e_msg = "Echospacing interval should be between 1.0 and 0.0 (off)"
		data["echospacing"].SetError(e_msg)
	} else {
		data["echospacing"].SetError("")
		args.echospace = es
	}

	fd, err := strconv.ParseBool(data["flip draw"].Get())
	if err != nil {
		valid = false
		e_msg = "Flip draw should be either 'false' or 'true'"
		data["flip draw"].SetError(e_msg)
	} else {
		data["flip draw"].SetError("")
		args.flip_draw = fd
	}

	is, err := strconv.ParseBool(data["intermediate steps"].Get())
	if err != nil {
		valid = false
		e_msg = "Intermediate steps should be either 'false' or 'true'"
		data["intermediate steps"].SetError(e_msg)
	} else {
		data["intermediate steps"].SetError("")
		args.draw_ir = is
	}

	sc, err := strconv.ParseInt(data["seed colour"].Get(), 0, 0)
	if err != nil || sc < 0 || sc > 0xFFFFFF {
		valid = false
		e_msg = "Colour should be a 24 bit hex, e.g. '0xFFFFFF'"
		data["seed colour"].SetError(e_msg)
	} else {
		data["seed colour"].SetError("")
		args.start_red = int(sc) >> 16
		args.start_green = (int(sc) & 0x00FF00) >> 8
		args.start_blue = int(sc) & 0x0000FF
	}

	w, err := strconv.Atoi(data["width"].Get())
	if err != nil || w < 0 || w > 4096 {
		valid = false
		e_msg = "Width should be between 0 and 4096"
		data["width"].SetError(e_msg)
		args.width = w
	} else {
		data["width"].SetError("")
		args.width = w
	}

	h, err := strconv.Atoi(data["height"].Get())
	if err != nil || h < 0 || h > 4096 {
		valid = false
		e_msg = "Height should be between 0 and 4096"
		data["height"].SetError(e_msg)
		args.height = h
	} else {
		data["height"].SetError("")
		args.height = h
	}

	sx, err := strconv.Atoi(data["start X"].Get())
	if err != nil || sx < 0 || sx >= args.width {
		valid = false
		e_msg = fmt.Sprintf("Start X should be between 0 and %d", args.width-1)
		data["start X"].SetError(e_msg)
	} else {
		data["start X"].SetError("")
		args.start_x = sx
	}

	sy, err := strconv.Atoi(data["start Y"].Get())
	if err != nil || sy < 0 || sy >= args.height {
		valid = false
		e_msg = fmt.Sprintf("Start Y should be between 0 and %d", args.height-1)
		data["start Y"].SetError(e_msg)
	} else {
		data["start Y"].SetError("")
		args.start_y = sy
	}

	args.tag = data["tag"].Get()

	if valid {
		args.update = UpdateProgress
	} else {
		args.update = nil
	}

	return
}

func UpdateProgress(pic image.Image) {
	Driver.Call(func(){
		texture := Driver.CreateTexture(pic, 0.125)
		ProgPic.SetTexture(texture)
	})
}

func GUImain() {
	FieldNames = []string{
		"chan size",
		"blur",
		"cpus",			// 1- MAX_CPUS (8?)
		"update freq",
		"colour basis",	// any of rgb
		"echospacing",	// float between 0 and 1 determining how much to progress through set before ES
		"flip draw", 	// technically a bool
		"intermediate steps",	// also a bool
		"seed colour",	// any of 0x000000 - 0xFFFFFF
		//"seed chroma",	// colour to ignore in initial image
		//"seed duplicates", // bool; attemp to reseed seen colours
		//"seed image",	// uploaded image
		//"seed culling rate", // % of pixels to reject from seed image
		"start X",
		"start Y",
		"tag",
		"width",
		"height",
	}
	gl.StartDriver(appMain)

}
