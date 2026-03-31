package target

import (
	"bytes"
	"context"
	"fmt"
	"image/color"
	"image/png"
	"net/http"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"bananas-pos/internal/job"
	jobtransform "bananas-pos/internal/transform"
)

type Emulator struct {
	client  *http.Client
	dpmm    int
	jobs    chan job.PrintJob
	done    chan struct{}
	ctx     context.Context
	cancel  context.CancelFunc
	onClose func()

	window           fyne.Window
	header           *fyne.Container
	stack            *fyne.Container
	scroll           *container.Scroll
	clearButton      *widget.Button
	width            float32
	defaultWidth     float32
	maxHeight        float32
	defaultMaxHeight float32

	mu          sync.Mutex
	placeholder fyne.CanvasObject
	wg          sync.WaitGroup
}

const (
	windowWidthSlackPx      = 10
	defaultEmulatorWidthPx  = 228
	defaultEmulatorHeightPx = 140
	emulatorHTTPTimeout     = 10 * time.Second
)

func NewEmulator(app fyne.App, icon fyne.Resource, dpmm int, onClose func()) *Emulator {
	header := container.NewVBox()
	stack := container.NewVBox()
	scroll := container.NewVScroll(stack)
	ctx, cancel := context.WithCancel(context.Background())
	defaultHeightMM := float64(defaultEmulatorHeightPx*2) / float64(dpmm)
	emulator := &Emulator{
		client:           &http.Client{Timeout: emulatorHTTPTimeout},
		ctx:              ctx,
		cancel:           cancel,
		dpmm:             dpmm,
		jobs:             make(chan job.PrintJob, 128),
		done:             make(chan struct{}),
		onClose:          onClose,
		header:           header,
		stack:            stack,
		scroll:           scroll,
		width:            defaultEmulatorWidthPx,
		defaultWidth:     defaultEmulatorWidthPx,
		maxHeight:        float32(defaultHeightMM * float64(dpmm) * 2.5),
		defaultMaxHeight: float32(defaultHeightMM * float64(dpmm) * 2.5),
	}

	window := app.NewWindow("Printer Emulator")
	window.SetIcon(icon)
	window.Resize(fyne.NewSize(
		emulator.windowWidth(),
		1,
	))
	window.SetFixedSize(true)
	clearButton := widget.NewButton("Tear", func() {
		emulator.clear()
	})
	clearButton.Hide()
	emulator.clearButton = clearButton
	window.SetContent(container.NewBorder(
		header,
		nil,
		nil,
		nil,
		scroll,
	))
	window.SetCloseIntercept(func() {
		if emulator.onClose != nil {
			emulator.onClose()
		}
	})
	emulator.window = window
	emulator.wg.Add(1)
	go emulator.run()

	return emulator
}

func (e *Emulator) Name() string {
	return "emulator"
}

func (e *Emulator) Send(ctx context.Context, printJob job.PrintJob) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-e.done:
		return context.Canceled
	case e.jobs <- printJob:
		return nil
	}
}

func (e *Emulator) Health(context.Context) error {
	return nil
}

func (e *Emulator) ShowWindow() {
	e.window.Show()
	e.window.RequestFocus()
}

func (e *Emulator) Start() error {
	e.ShowWindow()
	return nil
}

func (e *Emulator) clear() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.stack.Objects = nil
	e.stack.Refresh()
	e.scroll.ScrollToTop()
	e.header.Objects = nil
	e.header.Refresh()
	e.clearButton.Hide()
	e.width = e.defaultWidth
	e.maxHeight = e.defaultMaxHeight
	e.window.Resize(fyne.NewSize(e.windowWidth(), 1))
}

func (e *Emulator) Shutdown() error {
	close(e.done)
	e.cancel()
	e.wg.Wait()
	e.window.Hide()
	return nil
}

func (e *Emulator) run() {
	defer e.wg.Done()

	for {
		select {
		case <-e.done:
			return
		case printJob := <-e.jobs:
			if err := e.processJob(printJob); err != nil {
				fmt.Printf("emulator preview failed for %s: %v\n", printJob.ID, err)
			}

			timer := time.NewTimer(750 * time.Millisecond)
			select {
			case <-e.done:
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
}

func (e *Emulator) previewSize(widthPx, heightPx int, widthMM, heightMM float64) fyne.Size {
	targetWidth := float32(widthMM * float64(e.dpmm) / 2)
	targetHeight := float32(heightMM * float64(e.dpmm) / 2)

	if widthPx <= 0 || heightPx <= 0 {
		return fyne.NewSize(targetWidth, targetHeight)
	}

	scaleX := targetWidth / float32(widthPx)
	scaleY := targetHeight / float32(heightPx)
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}

	return fyne.NewSize(float32(widthPx)*scale, float32(heightPx)*scale)
}

func (e *Emulator) processJob(printJob job.PrintJob) error {
	labelWidthMM, labelHeightMM := jobtransform.LabelSizeMM(string(printJob.Raw), e.dpmm)

	pngBytes, err := jobtransform.FetchLabelaryPreview(e.ctx, e.client, string(printJob.Raw), e.dpmm)
	if err != nil {
		return err
	}

	cfg, err := png.DecodeConfig(bytes.NewReader(pngBytes))
	if err != nil {
		return fmt.Errorf("decode preview png: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if e.placeholder != nil {
		e.stack.Remove(e.placeholder)
		e.placeholder = nil
	}

	image := canvas.NewImageFromResource(fyne.NewStaticResource(printJob.ID+".png", pngBytes))
	image.FillMode = canvas.ImageFillContain
	image.SetMinSize(e.previewSize(cfg.Width, cfg.Height, labelWidthMM, labelHeightMM))

	previewWidth := float32(labelWidthMM * float64(e.dpmm) / 2)
	if previewWidth > e.width {
		e.width = previewWidth
	}
	previewMaxHeight := float32(labelHeightMM * float64(e.dpmm) * 2.5)
	if previewMaxHeight > e.maxHeight {
		e.maxHeight = previewMaxHeight
	}

	objects := []fyne.CanvasObject{image}
	if len(e.stack.Objects) > 0 {
		objects = append(objects, newDashedSeparator(e.width))
	}

	e.stack.Objects = append(objects, e.stack.Objects...)
	e.stack.Refresh()
	e.scroll.ScrollToTop()
	e.clearButton.Show()
	e.header.Objects = []fyne.CanvasObject{container.NewPadded(e.clearButton)}
	e.header.Refresh()
	e.resizeWindowToContent()
	e.ShowWindow()
	return nil
}

func (e *Emulator) resizeWindowToContent() {
	const windowSlack float32 = 8

	scrollSlack := theme.Padding()*2 + 2

	targetHeight := e.header.MinSize().Height + e.stackHeight() + scrollSlack + windowSlack
	if targetHeight > e.maxHeight {
		targetHeight = e.maxHeight
	}
	if targetHeight < 1 {
		targetHeight = 1
	}

	e.window.Resize(fyne.NewSize(e.windowWidth(), targetHeight))
}

func (e *Emulator) stackHeight() float32 {
	if len(e.stack.Objects) == 0 {
		return 0
	}

	height := float32(0)
	for _, object := range e.stack.Objects {
		height += object.MinSize().Height
	}

	height += theme.Padding() * float32(len(e.stack.Objects)-1)
	height += float32(len(e.stack.Objects))
	return height
}

func (e *Emulator) windowWidth() float32 {
	return e.width + windowWidthSlackPx
}

func newDashedSeparator(width float32) fyne.CanvasObject {
	const (
		dashWidth  float32 = 8
		gapWidth   float32 = 6
		lineHeight float32 = 1
	)

	segments := make([]fyne.CanvasObject, 0)
	for x := float32(0); x < width; x += dashWidth + gapWidth {
		end := x + dashWidth
		if end > width {
			end = width
		}
		line := canvas.NewLine(color.NRGBA{R: 120, G: 120, B: 120, A: 255})
		line.Position1 = fyne.NewPos(x, 0)
		line.Position2 = fyne.NewPos(end, 0)
		line.StrokeWidth = lineHeight
		segments = append(segments, line)
	}

	separator := container.NewWithoutLayout(segments...)
	separator.Resize(fyne.NewSize(width, lineHeight))
	return separator
}
