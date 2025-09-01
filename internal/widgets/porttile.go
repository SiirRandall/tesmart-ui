package widgets

import (
	"image/color"
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type PortTile struct {
	widget.BaseWidget
	bg       *canvas.Rectangle
	img      *canvas.Image
	label    *widget.Label
	content  *fyne.Container
	PortNum  int
	Selected bool
	IconRes  fyne.Resource
	IconSize fyne.Size
	OnTap    func()
}

func NewPortTile(port int, name string, icon fyne.Resource, onTap func()) *PortTile {
	if icon == nil {
		icon = theme.ComputerIcon()
	}
	bg := canvas.NewRectangle(color.NRGBA{R: 30, G: 30, B: 30, A: 255})
	bg.CornerRadius = 16

	img := canvas.NewImageFromResource(icon)
	img.FillMode = canvas.ImageFillContain
	iconSz := fyne.NewSize(65, 65)
	img.SetMinSize(iconSz)

	lbl := widget.NewLabel(name + "\n(#" + strconv.Itoa(port) + ")")
	lbl.Alignment = fyne.TextAlignCenter
	lbl.Wrapping = fyne.TextWrapWord

	inner := container.NewVBox(
		layout.NewSpacer(),
		container.NewCenter(img),
		layout.NewSpacer(),
		container.NewPadded(lbl),
	)
	content := container.NewMax(bg, container.NewPadded(inner))

	t := &PortTile{
		bg: bg, img: img, label: lbl, content: content,
		PortNum: port, IconRes: icon, IconSize: iconSz, OnTap: onTap,
	}
	t.ExtendBaseWidget(t)
	t.SetSelected(false)
	return t
}

func (t *PortTile) SetSelected(sel bool) {
	t.Selected = sel
	if sel {
		t.bg.FillColor = color.NRGBA{R: 0, G: 120, B: 255, A: 255}
	} else {
		t.bg.FillColor = color.NRGBA{R: 60, G: 60, B: 60, A: 255}
	}
	t.bg.Refresh()
}

func (t *PortTile) SetNameIcon(name string, icon fyne.Resource) {
	if icon == nil {
		icon = theme.ComputerIcon()
	}
	t.IconRes = icon
	t.img.Resource = icon
	t.img.SetMinSize(t.IconSize)
	t.img.Refresh()
	t.label.SetText(name + "\n(#" + strconv.Itoa(t.PortNum) + ")")
}

func (t *PortTile) Tapped(*fyne.PointEvent) {
	if t.OnTap != nil {
		t.OnTap()
	}
}
func (t *PortTile) TappedSecondary(*fyne.PointEvent) {}
func (t *PortTile) CreateRenderer() fyne.WidgetRenderer {
	objects := []fyne.CanvasObject{t.content}
	return &tileRenderer{tile: t, objects: objects}
}

type tileRenderer struct {
	tile    *PortTile
	objects []fyne.CanvasObject
}

func (r *tileRenderer) Layout(size fyne.Size)        { r.objects[0].Resize(size) }
func (r *tileRenderer) MinSize() fyne.Size           { return fyne.NewSize(170, 140) }
func (r *tileRenderer) Refresh()                     { canvas.Refresh(r.tile) }
func (r *tileRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *tileRenderer) Destroy()                     {}
