// Copyright © 2016-2023 Wei Shen <shenwei356@gmail.com>
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package cmd

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/shenwei356/util/stringutil"
	"github.com/spf13/cobra"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/plotutil"
	"gonum.org/v1/plot/vg"
)

// lineCmd represents the line command
var lineCmd = &cobra.Command{
	Use:   "line",
	Short: "line plot and scatter plot",
	Long: `line plot and scatter plot

Notes:

  1. Output file can be set by flag -o/--out-file.
  2. File format is determined by the out file suffix.
     Supported formats: eps, jpg|jpeg, pdf, png, svg, and tif|tiff
  3. If flag -o/--out-file not set (default), image is written to stdout,
     you can display the image by pipping to "display" command of Imagemagic
     or just redirect to file.

`,
	Run: func(cmd *cobra.Command, args []string) {
		config := getConfigs(cmd)
		plotConfig := getPlotConfigs(cmd)

		files := getFileListFromArgsAndFile(cmd, args, true, "infile-list", true)
		if len(files) > 1 {
			checkError(fmt.Errorf("no more than one file should be given"))
		}
		runtime.GOMAXPROCS(config.NumCPUs)

		scatter := getFlagBool(cmd, "scatter")
		lineWidth := vg.Points(getFlagPositiveFloat64(cmd, "line-width") * plotConfig.scale)
		pointSize := vg.Length(getFlagPositiveFloat64(cmd, "point-size") * plotConfig.scale)
		colorIndex := getFlagPositiveInt(cmd, "color-index")
		if colorIndex > 7 {
			checkError(fmt.Errorf("unsupported color index"))
		}

		dataFieldXStr := getFlagString(cmd, "data-field-x")
		if dataFieldXStr == "" {
			checkError(fmt.Errorf("flag -x (--data-field-x) needed"))
		}
		if strings.Contains(dataFieldXStr, ",") {
			checkError(fmt.Errorf("only one field allowed for flag -x (--data-field-x)"))
		}
		if dataFieldXStr[0] == '-' {
			checkError(fmt.Errorf("unselect not allowed for flag -y (--data-field-x)"))
		}

		dataFieldYStr := getFlagString(cmd, "data-field-y")
		if dataFieldYStr == "" {
			checkError(fmt.Errorf("flag -y (--data-field-y) needed"))
		}
		if strings.Contains(dataFieldYStr, ",") {
			checkError(fmt.Errorf("only one field allowed for flag -y (--data-field-y)"))
		}
		if dataFieldXStr[0] == '-' {
			checkError(fmt.Errorf("unselect not allowed for flag -y (--data-field-y)"))
		}

		groupFieldStr := getFlagString(cmd, "group-field")
		if len(groupFieldStr) > 0 {
			if strings.Contains(groupFieldStr, ",") {
				checkError(fmt.Errorf("only one field allowed for flag --group-field"))
			}
			if groupFieldStr[0] == '-' {
				checkError(fmt.Errorf("unselect not allowed for flag --group-field"))
			}
			plotConfig.fieldStr = dataFieldXStr + "," + dataFieldYStr + "," + groupFieldStr
		} else {
			plotConfig.fieldStr = dataFieldXStr + "," + dataFieldYStr
		}

		skipNA := getFlagBool(cmd, "skip-na")
		naValues := getFlagStringSlice(cmd, "na-values")
		if skipNA && len(naValues) == 0 {
			log.Errorf("the value of --na-values should not be empty when using --skip-na")
		}
		naMap := make(map[string]interface{}, len(naValues))
		for _, na := range naValues {
			naMap[strings.ToLower(na)] = struct{}{}
		}

		file := files[0]
		headerRow, fields, data, _, _, err := parseCSVfile(cmd, config, file, plotConfig.fieldStr, false, true, false)

		if err != nil {
			// if err == xopen.ErrNoContent {
			// 	log.Warningf("csvtk box: skipping empty input file: %s", file)
			// 	return
			// }
			checkError(err)
		}

		// =======================================

		groups := make(map[string]plotter.XYs)
		groupOrderMap := make(map[string]int)
		var x, y float64
		var ok bool
		var order int
		var groupName string
		for _, d := range data {
			if skipNA {
				if _, ok = naMap[strings.ToLower(d[0])]; ok {
					continue
				}
			}

			x, err = strconv.ParseFloat(d[0], 64)
			if err != nil {
				if len(headerRow) > 0 {
					checkError(fmt.Errorf("fail to parse X value: %s at column: %s. please choose the right column by flag --data-field-x", d[0], headerRow[0]))
				} else {
					checkError(fmt.Errorf("fail to parse X value: %s at column: %d. please choose the right column by flag --data-field-x", d[0], fields[0]))
				}
			}
			if len(d) > 1 {
				if skipNA {
					if _, ok = naMap[strings.ToLower(d[1])]; ok {
						continue
					}
				}
				y, err = strconv.ParseFloat(d[1], 64)
			} else {
				y, err = strconv.ParseFloat(d[0], 64)
			}
			if err != nil {
				if len(headerRow) > 0 {
					checkError(fmt.Errorf("fail to parse Y value: %s at column: %s. please choose the right column by flag --data-field-y", d[1], headerRow[1]))
				} else {
					checkError(fmt.Errorf("fail to parse Y value: %s at column: %d. please choose the right column by flag --data-field-y", d[1], fields[1]))
				}
			}

			if len(d) > 2 {
				groupName = d[2]
			} else {
				groupName = ""
			}
			if _, ok = groups[groupName]; !ok {
				groups[groupName] = make(plotter.XYs, 0)
			}
			groups[groupName] = append(groups[groupName], struct{ X, Y float64 }{X: x, Y: y})
			if _, ok = groupOrderMap[groupName]; !ok {
				groupOrderMap[groupName] = order
				order++
			}
		}

		p := plot.New()

		var groupOrders []stringutil.StringCount
		for g := range groupOrderMap {
			groupOrders = append(groupOrders, stringutil.StringCount{Key: g, Count: groupOrderMap[g]})
		}
		sort.Sort(stringutil.StringCountList(groupOrders))

		i := colorIndex - 1
		addLegend := len(groupOrders) > 1
		for _, gor := range groupOrders {
			v := groups[gor.Key]

			// BYPASS sort by x
			// if !scatter {
			// 	sort.Slice(v, func(i, j int) bool {
			// 		return v[i].X < v[j].X
			// 	})
			// }

			g := gor.Key
			if !scatter {
				lines, points, err := plotter.NewLinePoints(v)
				checkError(err)
				fmt.Fprintln(os.Stderr, i)
				lines.Color = plotutil.Color(i)
				lines.LineStyle.Dashes = plotutil.Dashes(i)
				lines.LineStyle.Width = lineWidth
				points.Shape = plotutil.Shape(i)
				points.Color = plotutil.Color(i)
				points.Radius = pointSize
				p.Add(lines, points)
				if addLegend {
					p.Legend.Add(g, lines, points)
				}
			} else {
				points, err := plotter.NewScatter(v)
				checkError(err)
				points.Shape = plotutil.Shape(i)
				points.Color = plotutil.Color(i)
				points.Radius = pointSize
				p.Add(points)
				if addLegend {
					p.Legend.Add(g, points)
				}
			}

			i++
		}
		if lineWidth > pointSize {
			p.Legend.Padding = vg.Length(lineWidth)
		} else {
			p.Legend.Padding = vg.Length(pointSize)
		}
		p.Legend.Top = getFlagBool(cmd, "legend-top")
		p.Legend.Left = getFlagBool(cmd, "legend-left")

		if plotConfig.ylab == "" {
			if len(headerRow) > 1 {
				plotConfig.ylab = headerRow[1]
			} else {
				plotConfig.ylab = "Y Values"
			}
		}
		if plotConfig.xlab == "" {
			if len(headerRow) > 0 {
				plotConfig.xlab = headerRow[0]
			} else {
				plotConfig.xlab = "X Values"
			}
		}

		p.Title.Text = plotConfig.title
		p.Title.TextStyle.Font.Size = plotConfig.titleSize
		p.X.Label.Text = plotConfig.xlab
		p.Y.Label.Text = plotConfig.ylab
		p.X.Label.TextStyle.Font.Size = plotConfig.labelSize
		p.Y.Label.TextStyle.Font.Size = plotConfig.labelSize
		p.X.Width = plotConfig.axisWidth
		p.Y.Width = plotConfig.axisWidth
		p.X.Tick.Width = plotConfig.tickWidth
		p.Y.Tick.Width = plotConfig.tickWidth
		p.X.Tick.Label.Font.Size = plotConfig.tickLabelSize
		p.Y.Tick.Label.Font.Size = plotConfig.tickLabelSize

		if plotConfig.xminStr != "" {
			p.X.Min = plotConfig.xmin
		}
		if plotConfig.xmaxStr != "" {
			p.X.Max = plotConfig.xmax
		}
		if plotConfig.yminStr != "" {
			p.Y.Min = plotConfig.ymin
		}
		if plotConfig.ymaxStr != "" {
			p.Y.Max = plotConfig.ymax
		}

		// Save image
		if isStdin(config.OutFile) {
			fh, err := p.WriterTo(plotConfig.width*vg.Inch,
				plotConfig.height*vg.Inch,
				plotConfig.format)
			checkError(err)
			_, err = fh.WriteTo(os.Stdout)
			checkError(err)
		} else {
			checkError(p.Save(plotConfig.width*vg.Inch,
				plotConfig.height*vg.Inch,
				config.OutFile))
		}
	},
}

func init() {
	plotCmd.AddCommand(lineCmd)
	lineCmd.Flags().StringP("data-field-x", "x", "", `column index or column name of X for command line`)
	lineCmd.Flags().StringP("data-field-y", "y", "", `column index or column name of Y for command line`)

	lineCmd.Flags().BoolP("legend-top", "", false, "locate legend along the top edge of the plot")
	lineCmd.Flags().BoolP("legend-left", "", false, "locate legend along the left edge of the plot")

	lineCmd.Flags().BoolP("scatter", "", false, "only plot points")
	lineCmd.Flags().Float64P("line-width", "", 1.5, "line width")
	lineCmd.Flags().Float64P("point-size", "", 3, "point size")
	lineCmd.Flags().IntP("color-index", "", 1, `color index, 1-7`)
}
