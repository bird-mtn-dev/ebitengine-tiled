package etiled

import (
	"encoding/xml"
	"fmt"
	"image"
	_ "image/png"
	"io"
	"io/fs"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

type Property struct {
	Name  string `xml:"name,attr"`
	Type  string `xml:"type,attr"`
	Value string `xml:"value,attr"`
}

type TileMap struct {
	XMLName       xml.Name `xml:"map"`
	Version       string   `xml:"version,attr"`
	TiledVersion  string   `xml:"tiledversion,attr"`
	Path          string
	Orientation   string               `xml:"orientation,attr"`
	RenderOrder   string               `xml:"renderorder,attr"`
	Width         int                  `xml:"width,attr"`
	Height        int                  `xml:"height,attr"`
	TileWidth     int                  `xml:"tilewidth,attr"`
	TileHeight    int                  `xml:"tileheight,attr"`
	Infinite      bool                 `xml:"infinite,attr"`
	TileSets      []TileSetDefinition  `xml:"tileset"`
	GroupLayers   []GroupLayer         `xml:"group"`
	Layers        []Layer              `xml:"layer"`
	ObjectGroups  []TileMapObjectGroup `xml:"objectgroup"`
	Properties    []Property           `xml:"properties>property"`
	currentTick   int
	allLayers     []*Layer
	allGroups     []*TileMapObjectGroup
	AllCollisions []*TileSetObject
	Zoom          float64
}

type TileSetDefinition struct {
	XMLName  xml.Name `xml:"tileset"`
	FirstGID int      `xml:"firstgid,attr"`
	Source   string   `xml:"source,attr"`
	TileSet  *TileSet
}

type GroupLayer struct {
	XMLName      xml.Name             `xml:"group"`
	Name         string               `xml:"name,attr"`
	Layers       []Layer              `xml:"layer"`
	ObjectGroups []TileMapObjectGroup `xml:"objectgroup"`
	GroupLayers  []GroupLayer         `xml:"group"`
	Properties   []Property           `xml:"properties>property"`
	Tilemap      *TileMap
}

func (gl *GroupLayer) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "name" {
			gl.Name = attr.Value
		}
	}

	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if tok.Name.Local == "layer" {
				_ = d.DecodeElement(&gl.Layers, &tok)
			} else if tok.Name.Local == "group" {
				_ = d.DecodeElement(&gl.GroupLayers, &tok)
			} else if tok.Name.Local == "objectgroup" {
				_ = d.DecodeElement(&gl.ObjectGroups, &tok)
			}
		}
	}

	return nil
}

func (groupLayer *GroupLayer) Draw(screen *ebiten.Image) {
	tileMap := groupLayer.Tilemap

	for idx := range groupLayer.Layers {
		groupLayer.Layers[idx].Tilemap = tileMap
		groupLayer.Layers[idx].Draw(screen)
	}

}

type Layer struct {
	XMLName    xml.Name   `xml:"layer"`
	Id         int        `xml:"id,attr"`
	Name       string     `xml:"name,attr"`
	Width      int        `xml:"width,attr"`
	Height     int        `xml:"height,attr"`
	Properties []Property `xml:"properties>property"`
	Data       Data       `xml:"data"`
	Tilemap    *TileMap
}

func (l *Layer) GetId() string {
	return l.Name
}

func (l *Layer) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "id" {
			l.Id, _ = strconv.Atoi(attr.Value)
		} else if attr.Name.Local == "name" {
			l.Name = attr.Value
		} else if attr.Name.Local == "width" {
			l.Width, _ = strconv.Atoi(attr.Value)
		} else if attr.Name.Local == "height" {
			l.Height, _ = strconv.Atoi(attr.Value)
		}
	}

	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if tok.Name.Local == "data" {
				_ = d.DecodeElement(&l.Data, &tok)
			}
		}
	}

	return nil
}

func (layer *Layer) Draw(screen *ebiten.Image) {
	tileMap := layer.Tilemap

	for y, row := range layer.Data.Values {
		for x, tileId := range row {
			if tileId == 0 {
				continue
			}
			tileSet, offset := tileMap.GetTileSetByTileId(tileId)
			tileId = tileId - offset

			var tileData *Tile
			for idx := range tileSet.Tiles {
				if tileSet.Tiles[idx].Id == tileId {
					tileData = &tileSet.Tiles[idx]
					break
				}
			}
			if tileData != nil && len(tileData.Animation) > 0 {
				var frameIdx = 0
				var effectiveTick int
				if tileData.PreviousFrame != nil {
					if tileMap.currentTick < tileData.PreviousTick {
						effectiveTick = tileMap.currentTick + ((ebiten.TPS() * 10) - tileData.PreviousTick)
					} else {
						effectiveTick = tileMap.currentTick - tileData.PreviousTick
					}
					if float64(effectiveTick)/float64(ebiten.TPS()) > (float64(tileData.Animation[*tileData.PreviousFrame].Duration) / 1000) {
						frameIdx = (*tileData.PreviousFrame + 1) % len(tileData.Animation)
						tileData.PreviousFrame = &frameIdx
						tileData.PreviousTick = tileMap.currentTick
					}
				} else {
					tileData.PreviousFrame = &frameIdx
					tileData.PreviousTick = tileMap.currentTick
				}
				tileId = tileData.Animation[*tileData.PreviousFrame].TileId
			}

			tlX := float64(x * tileMap.TileWidth)
			tlY := float64(y * tileMap.TileHeight)

			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(tileMap.Zoom, tileMap.Zoom)
			op.GeoM.Translate(tlX, tlY)

			sx := (tileId % tileSet.Columns) * tileSet.TileWidth
			sy := (tileId / tileSet.Columns) * tileSet.TileHeight
			screen.DrawImage(tileSet.Image.LoadedImage.SubImage(image.Rect(sx, sy, sx+tileSet.TileWidth, sy+tileSet.TileHeight)).(*ebiten.Image), op)

		}
	}
}

type TileMapObjectGroup struct {
	XMLName    xml.Name        `xml:"objectgroup"`
	Id         int             `xml:"id,attr"`
	Name       string          `xml:"name,attr"`
	Properties []Property      `xml:"properties>property"`
	Objects    []TileMapObject `xml:"object"`
	Tilemap    *TileMap
}

func (o *TileMapObjectGroup) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "id" {
			o.Id, _ = strconv.Atoi(attr.Value)
		} else if attr.Name.Local == "name" {
			o.Name = attr.Value
		}
	}

	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if tok.Name.Local == "object" {
				_ = d.DecodeElement(&o.Objects, &tok)
			}
		}
	}
	return nil
}

func (o *TileMapObjectGroup) GetId() string {
	return o.Name
}

func (objectGroup *TileMapObjectGroup) Draw(screen *ebiten.Image) {
	tileMap := objectGroup.Tilemap

	for _, object := range objectGroup.Objects {
		tileId := object.Gid
		if tileId == 0 {
			continue
		}
		tileSet, offset := tileMap.GetTileSetByTileId(tileId)
		tileId = tileId - offset

		var tileData *Tile
		for idx := range tileSet.Tiles {
			if tileSet.Tiles[idx].Id == tileId {
				tileData = &tileSet.Tiles[idx]
				break
			}
		}
		if tileData != nil && len(tileData.Animation) > 0 {
			var frameIdx = 0
			var effectiveTick int
			if tileData.PreviousFrame != nil {
				if tileMap.currentTick < tileData.PreviousTick {
					effectiveTick = tileMap.currentTick + ((ebiten.TPS() * 10) - tileData.PreviousTick)
				} else {
					effectiveTick = tileMap.currentTick - tileData.PreviousTick
				}
				if float64(effectiveTick)/float64(ebiten.TPS()) > (float64(tileData.Animation[*tileData.PreviousFrame].Duration) / 1000) {
					frameIdx = (*tileData.PreviousFrame + 1) % len(tileData.Animation)
					tileData.PreviousFrame = &frameIdx
					tileData.PreviousTick = tileMap.currentTick
				}
			} else {
				tileData.PreviousFrame = &frameIdx
				tileData.PreviousTick = tileMap.currentTick
			}
			tileId = tileData.Animation[*tileData.PreviousFrame].TileId
		}

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(object.Height)/float64(tileSet.TileHeight), float64(object.Width)/float64(tileSet.TileWidth))
		op.GeoM.Scale(tileMap.Zoom, tileMap.Zoom)
		op.GeoM.Translate(object.X, object.Y-float64(tileSet.TileHeight))

		sx := (tileId % tileSet.Columns) * tileSet.TileWidth
		sy := (tileId / tileSet.Columns) * tileSet.TileHeight
		screen.DrawImage(tileSet.Image.LoadedImage.SubImage(image.Rect(sx, sy, sx+tileSet.TileWidth, sy+tileSet.TileHeight)).(*ebiten.Image), op)

	}
}

type TileMapObject struct {
	XMLName xml.Name `xml:"object"`
	Id      int      `xml:"id,attr"`
	Gid     int      `xml:"gid,attr"`
	X       float64  `xml:"x,attr"`
	Y       float64  `xml:"y,attr"`
	Width   int      `xml:"width,attr"`
	Height  int      `xml:"height,attr"`
}

type Data struct {
	Encoding string
	Values   [][]int
}

func (s *Data) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var content string
	if err := d.DecodeElement(&content, &start); err != nil {
		return err
	}
	for _, attr := range start.Attr {
		if attr.Name.Local == "encoding" {
			s.Encoding = attr.Value
		}
	}
	content = strings.TrimSpace(content)
	rows := strings.Split(content, "\n")
	for _, row := range rows {
		idStrings := strings.Split(row, ",")
		var rowIds []int
		for _, i := range idStrings {
			if i == "" {
				continue
			}
			j, err := strconv.Atoi(i)
			if err != nil {
				panic(err)
			}
			rowIds = append(rowIds, j)
		}
		s.Values = append(s.Values, rowIds)
	}
	return nil
}

type TileSet struct {
	XMLName      xml.Name `xml:"tileset"`
	Version      string   `xml:"version,attr"`
	TiledVersion string   `xml:"tiledversion,attr"`
	Name         string   `xml:"name,attr"`
	Path         string
	TileWidth    int             `xml:"tilewidth,attr"`
	TileHeight   int             `xml:"tileheight,attr"`
	TileCount    int             `xml:"tilecount,attr"`
	Columns      int             `xml:"columns,attr"`
	Image        ImageDefinition `xml:"image"`
	Properties   []Property      `xml:"properties>property"`
	Tiles        []Tile          `xml:"tile"`
}

func (tileSet *TileSet) GetTileById(id int) *Tile {
	for idx := range tileSet.Tiles {
		if tileSet.Tiles[idx].Id == id {
			return &tileSet.Tiles[idx]
		}
	}
	return nil
}

type ImageDefinition struct {
	XMLName     xml.Name `xml:"image"`
	Source      string   `xml:"source,attr"`
	Width       int      `xml:"width,attr"`
	Height      int      `xml:"height,attr"`
	LoadedImage *ebiten.Image
}

type Tile struct {
	XMLName       xml.Name   `xml:"tile"`
	Id            int        `xml:"id,attr"`
	Properties    []Property `xml:"properties>property"`
	PreviousTick  int
	PreviousFrame *int
	Animation     []Frame            `xml:"animation>frame"`
	ObjectGroup   TileSetObjectGroup `xml:"objectgroup"`
}
type Frame struct {
	XMLName  xml.Name `xml:"frame"`
	TileId   int      `xml:"tileid,attr"`
	Duration int      `xml:"duration,attr"`
}

type TileSetObjectGroup struct {
	XMLName    xml.Name        `xml:"objectgroup"`
	Id         int             `xml:"id,attr"`
	DrawOrder  string          `xml:"draworder,attr"`
	Properties []Property      `xml:"properties>property"`
	Objects    []TileSetObject `xml:"object"`
}

type TileSetObject struct {
	XMLName    xml.Name    `xml:"object"`
	Id         int         `xml:"id,attr"`
	X          float64     `xml:"x,attr"`
	Y          float64     `xml:"y,attr"`
	Name       string      `xml:"name,attr"`
	Type       string      `xml:"type,attr"`
	Width      *float64    `xml:"width,attr"`
	Height     *float64    `xml:"height,attr"`
	Visible    *bool       `xml:"visible,attr"`
	IsPoint    *NodeExists `xml:"point"`
	IsEllipse  *NodeExists `xml:"ellipse"`
	Polygon    *Polygon    `xml:"polygon"`
	Properties []Property  `xml:"properties>property"`
}

type TileSetObjectType int

const (
	POINT TileSetObjectType = iota
	ELLIPSE
	POLYGON
	RECTANGLE
)

func (o *TileSetObject) GetType() TileSetObjectType {
	if o.IsPoint != nil {
		return POINT
	} else if o.IsEllipse != nil {
		return ELLIPSE
	} else if o.Polygon != nil {
		return POLYGON
	} else {
		return RECTANGLE
	}
}

type NodeExists struct {
}

type Polygon struct {
	XMLName xml.Name `xml:"polygon"`
	Points  []Point
}

type Point struct {
	X float64
	Y float64
}

func (s *Polygon) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var content string
	if err := d.DecodeElement(&content, &start); err != nil {
		return err
	}

	for _, attr := range start.Attr {
		if attr.Name.Local == "points" {
			content = attr.Value
			break
		}
	}
	content = strings.TrimSpace(content)
	points := strings.Split(content, " ")
	for _, point := range points {
		xy := strings.Split(point, ",")
		x, _ := strconv.ParseFloat(xy[0], 64)
		y, _ := strconv.ParseFloat(xy[1], 64)
		s.Points = append(s.Points, Point{X: x, Y: y})
	}
	return nil
}

func (tileMap *TileMap) GetTileSetByTileId(id int) (*TileSet, int) {
	for x := len(tileMap.TileSets) - 1; x >= 0; x-- {
		if id >= tileMap.TileSets[x].FirstGID {
			return tileMap.TileSets[x].TileSet, tileMap.TileSets[x].FirstGID
		}
	}

	return nil, -1
}

func (tileMap *TileMap) GetDimensions() (int, int) {
	return int(float64(tileMap.Width) * float64(tileMap.TileWidth) * tileMap.Zoom), int(float64(tileMap.Height) * float64(tileMap.TileHeight) * tileMap.Zoom)
}

func (tileMap *TileMap) Update() {
	tileMap.currentTick = tileMap.currentTick + 1
	tileMap.currentTick = tileMap.currentTick % (ebiten.TPS() * 10)
}

func (tileMap *TileMap) Draw(screen *ebiten.Image) {
	for _, layer := range tileMap.allLayers {
		layer.Draw(screen)
	}
}

func (tileMap *TileMap) GetLayerByName(name string) *Layer {
	for idx := range tileMap.allLayers {
		if tileMap.allLayers[idx].Name == name {
			return tileMap.allLayers[idx]
		}
	}
	return nil
}

func (tileMap *TileMap) GetObjectGroupByName(name string) *TileMapObjectGroup {
	for idx := range tileMap.allGroups {
		if tileMap.allGroups[idx].Name == name {
			return tileMap.allGroups[idx]
		}
	}
	return nil
}

func (tileMap *TileMap) GetGroupLayerByName(name string) *GroupLayer {
	for idx := range tileMap.GroupLayers {
		if tileMap.GroupLayers[idx].Name == name {
			return &tileMap.GroupLayers[idx]
		}
	}
	return nil
}

// This opens a .tmx file based on the current working directory.
//
// An example call would looke like: etiled.OpenTileMap("assets/tilemap/base.tmx")
func OpenTileMap(file string) *TileMap {
	workingDir, _ := os.Getwd()
	return OpenTileMapWithFileSystem(file, os.DirFS(workingDir))
}

func OpenTileMapWithFileSystem(file string, filesystem fs.FS) *TileMap {
	// Open our xmlFile
	xmlFile, err := filesystem.Open(file)
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
	}

	// defer the closing of our xmlFile so that we can parse it later on
	defer xmlFile.Close()

	// read our opened xmlFile as a byte array.
	byteValue, _ := io.ReadAll(xmlFile)

	// we initialize our Users array
	var tilemap TileMap
	tilemap.currentTick = 0
	tilemap.Path = file
	tilemap.Zoom = 1
	// we unmarshal our byteArray which contains our
	// xmlFiles content into 'users' which we defined above

	_ = xml.Unmarshal(byteValue, &tilemap)
	for i := range tilemap.TileSets {
		path := path.Join(path.Dir(file), tilemap.TileSets[i].Source)
		tilemap.TileSets[i].TileSet = openTileSet(path, filesystem)
	}
	tilemap.allLayers = merge(&tilemap.Layers)

	tilemap.walkTileMap()

	return &tilemap
}

func merge(layers *[]Layer) []*Layer {
	final := []*Layer{}
	layerIdx := 0

	for ; layerIdx < len(*layers); layerIdx++ {
		final = append(final, &(*layers)[layerIdx])
	}

	return final
}

func (tilemap *TileMap) walkTileMap() {
	for idx := range tilemap.Layers {
		tilemap.processLayer(&tilemap.Layers[idx])
	}
	for idx := range tilemap.ObjectGroups {
		tilemap.processObjectGroup(&tilemap.ObjectGroups[idx])
	}
	for idx := range tilemap.GroupLayers {
		tilemap.processGroupLayer(&tilemap.GroupLayers[idx])
	}
}

func (tilemap *TileMap) processLayer(layer *Layer) {
	layer.Tilemap = tilemap
	for y := 0; y < layer.Height; y += 1 {
		for x := 0; x < layer.Width; x += 1 {
			tileId := layer.Data.Values[y][x]
			tileset, gid := tilemap.GetTileSetByTileId(tileId)
			if tileset != nil {
				tile := tileset.GetTileById(tileId - gid)
				if tile != nil {
					for _, object := range tile.ObjectGroup.Objects {
						objToSave := object
						objToSave.X += float64(x * tilemap.TileWidth)
						objToSave.Y += float64(y * tilemap.TileHeight)
						tilemap.AllCollisions = append(tilemap.AllCollisions, &objToSave)
					}

				}
			}
		}
	}
}

func (tilemap *TileMap) processObjectGroup(tmog *TileMapObjectGroup) {
	tmog.Tilemap = tilemap
	for idx := range tmog.Objects {
		tileset, gid := tilemap.GetTileSetByTileId(tmog.Objects[idx].Gid)
		if tileset != nil {
			tile := tileset.GetTileById(tmog.Objects[idx].Gid - gid)
			if tile != nil {
				for _, object := range tile.ObjectGroup.Objects {
					objToSave := object
					objToSave.X += tmog.Objects[idx].X
					objToSave.Y += tmog.Objects[idx].Y
					tilemap.AllCollisions = append(tilemap.AllCollisions, &objToSave)
				}
			}
		}
	}
}

func (tilemap *TileMap) processGroupLayer(gl *GroupLayer) {
	gl.Tilemap = tilemap
	for idx := range gl.Layers {
		tilemap.processLayer(&gl.Layers[idx])
	}
	for idx := range gl.ObjectGroups {
		tilemap.processObjectGroup(&gl.ObjectGroups[idx])
	}
	for idx := range gl.GroupLayers {
		tilemap.processGroupLayer(&gl.GroupLayers[idx])
	}
}

func openTileSet(tsPath string, filesystem fs.FS) *TileSet {
	// Open our xmlFile
	xmlFile, err := filesystem.Open(tsPath)
	// if we os.Open returns an error then handle it
	if err != nil {
		fmt.Println(err)
		return nil
	}

	// defer the closing of our xmlFile so that we can parse it later on
	defer xmlFile.Close()

	// read our opened xmlFile as a byte array.
	byteValue, _ := io.ReadAll(xmlFile)

	// we initialize our Users array
	var tileset TileSet
	tileset.Path = tsPath
	// we unmarshal our byteArray which contains our
	// xmlFiles content into 'users' which we defined above
	_ = xml.Unmarshal(byteValue, &tileset)
	tileset.Image.LoadedImage = GetImageFromFilePath(path.Join(path.Dir(tileset.Path), tileset.Image.Source), filesystem)
	return &tileset
}

func GetImageFromFilePath(filePath string, filesystem fs.FS) *ebiten.Image {
	f, err := filesystem.Open(filePath)
	if err != nil {
		return nil
	}
	defer f.Close()
	image, _, err := image.Decode(f)
	if err != nil {
		panic(err)
	}
	return ebiten.NewImageFromImage(image)
}
