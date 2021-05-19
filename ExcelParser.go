package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/360EntSecGroup-Skylar/excelize"
)

func main() {
	// 프로그램 초기화
	Initialize()

	// Common 정보 로드
	GetCommonData()

	var wg sync.WaitGroup
	wg.Add(len(TableList))

	// 데이터 정보 로드
	for i := 0; i < len(TableList); i++ {
		go func(i int) {
			defer wg.Done()
			SetTableKeyValue(TableList[i])
		}(i)
	}
	wg.Wait()

	wg = sync.WaitGroup{}
	wg.Add(len(TableList))
	// 데이터 추출
	for i := 0; i < len(TableList); i++ {
		go func(i int) {
			defer wg.Done()
			ExtractTable(TableList[i])
		}(i)
	}
	wg.Wait()
	// fmt.Println(TableStruct)

	// Enum 및 테이블 구조 파일 추출
	CreateEnumFile()
	CreateTableFile()
}

// 0.Common.xlxs 파일
func GetCommonData() {
	file, err := excelize.OpenFile("0.Common.xlsx")
	HandleErr(err)

	// TableList 셋팅
	rows, err := file.GetRows("TableList")
	HandleErr(err)

	for row, rowData := range rows {
		for col, cellData := range rowData {
			if 2 <= row && 1 == col {
				TableList = append(TableList, cellData)
			}
		}
	}

	// Enum 셋팅
	cols, err := file.Cols("Enum")
	HandleErr(err)
	for cols.Next() {
		colData, err := cols.Rows()
		HandleErr(err)

		var enumString string
		for row, cellData := range colData {
			if row == 1 && cellData == "" { // 빈 컬럼 처리
				break
			}
			if row == 0 { // 첫 번째 빈 로우 처리
				continue
			}

			if row == 1 {
				enumColData := make(map[string]int)
				enumString = cellData
				EnumData[cellData] = enumColData

				EnumList = append(EnumList, cellData)
			} else {
				if enumColData, ok := EnumData[enumString]; ok {
					if cellData != "" {
						enumColData[cellData] = len(enumColData)
					}

				} else {
					ThrowErr(EnumErr.ERR_ENUM, "Enum")
				}
			}
		}
	}
}

// 테이블 Key:Value 설정
func SetTableKeyValue(tableName string) {
	tableKeyValue := make(map[string]int)
	TableLock.Lock()
	TableData[tableName] = tableKeyValue
	TableLock.Unlock()

	file, err := excelize.OpenFile(tableName + ".xlsx")
	HandleErr(err)

	rows, err := file.GetRows("Table")
	HandleErr(err)

	for row, rowData := range rows {
		var curValue int
		for col, cellData := range rowData {
			if col >= 3 {
				break
			}
			// 유효성 체크
			if row == 3 && col == 1 {
				if cellData != "Key" {
					ThrowErr(EnumErr.ERR_KEY, tableName)
				}
			} else if row == 3 && col == 2 {
				if cellData != "Name" {
					ThrowErr(EnumErr.ERR_NAME, tableName)
				}
			}

			if row > 3 && col == 1 {
				val, err := strconv.Atoi(cellData)
				HandleErr(err)
				curValue = val
			} else if row > 3 && col == 2 {
				TableLock.Lock()
				tableKeyValue[cellData] = curValue
				TableLock.Unlock()
			}
		}
	}
}

// 테이블 추출
func ExtractTable(tableName string) {
	file, err := excelize.OpenFile(tableName + ".xlsx")
	HandleErr(err)

	cols, err := file.Cols("Table")
	HandleErr(err)

	// 테이블 구조 셋팅
	columnInfoMap := make(map[int]ColumnInfo)
	TableStruct[tableName] = columnInfoMap

	var col int
	for cols.Next() {
		if col > 0 {
			colData, err := cols.Rows()
			HandleErr(err)

			var columnInfo ColumnInfo
			for row, cellData := range colData {
				if row == 0 {
					continue
				} else if row > 4 {
					break
				}

				if row == 1 { // 사용처
					if cellData != "All" && cellData != "Server" && cellData != "Client" && cellData != "None" {
						ThrowErr(EnumErr.ERR_USE_PLACE, tableName)
					}
					columnInfo.UsePlace = cellData
				} else if row == 2 { // 자료형
					// 배열 여부 체크
					beforeLen := len(cellData)
					cellData = strings.ReplaceAll(cellData, "[]", "")
					afterLen := len(cellData)
					if afterLen < beforeLen {
						columnInfo.IsArray = true
					}

					splitDatas := strings.Split(cellData, "/")
					if len(splitDatas) == 0 {
						ThrowErr(EnumErr.ERR_TYPE, tableName)
					}
					if splitDatas[0] == "Enum" {
						if len(splitDatas) < 2 {
							ThrowErr(EnumErr.ERR_TYPE, tableName)
						}
						if _, ok := EnumData[splitDatas[1]]; ok {
							columnInfo.Type = "Enum"
							columnInfo.TypeValue = splitDatas[1]
						} else {
							ThrowErr(EnumErr.ERR_ENUM, tableName)
						}
					} else if splitDatas[0] == "Table" {
						if len(splitDatas) < 2 {
							ThrowErr(EnumErr.ERR_TYPE, tableName)
						}
						var isExist bool
						for i := 0; i < len(TableList); i++ {
							if TableList[i] == splitDatas[1] {
								isExist = true
								break
							}
						}
						if isExist {
							columnInfo.Type = "Table"
							columnInfo.TypeValue = splitDatas[1]
						} else {
							ThrowErr(EnumErr.ERR_TYPE, tableName)
						}
					} else {
						if splitDatas[0] != "Int" && splitDatas[0] != "String" && splitDatas[0] != "Float" {
							ThrowErr(EnumErr.ERR_TYPE, tableName)
						}
						columnInfo.Type = splitDatas[0]
					}
				} else if row == 3 { // 변수명
					columnInfo.Name = cellData
				}
			}

			columnInfoMap[col-1] = columnInfo
		}
		col++
	}

	// Column 정보로 구조체 생성(reflection)
	serverStruct, clientStruct := CreateStruct(columnInfoMap)

	// Row 데이터 담기
	rows, err := file.GetRows("Table")
	HandleErr(err)

	var serverData []interface{}
	var clientData []interface{}

	for row, rowData := range rows {
		if row < 4 {
			continue
		}

		rowServerData := reflect.New(serverStruct).Elem()
		rowClientData := reflect.New(clientStruct).Elem()

		for col, cellData := range rowData {
			if col == 0 {
				continue
			}
			columnInfo := columnInfoMap[col-1]
			serverField := rowServerData.FieldByName(columnInfo.Name)
			clientField := rowClientData.FieldByName(columnInfo.Name)

			if columnInfo.Type == "Int" {
				if columnInfo.IsArray {
					cellDataList := strings.Split(cellData, "/")
					var sliceData []int
					for _, data := range cellDataList {
						val, err := strconv.Atoi(data)
						HandleErr(err)

						sliceData = append(sliceData, val)
					}

					if serverField.IsValid() {
						serverField.Set(reflect.ValueOf(sliceData))
					}
					if clientField.IsValid() {
						clientField.Set(reflect.ValueOf(sliceData))
					}
				} else {
					val, err := strconv.Atoi(cellData)
					HandleErr(err)
					if serverField.IsValid() {
						serverField.SetInt(int64(val))
					}
					if clientField.IsValid() {
						clientField.SetInt(int64(val))
					}
				}
			} else if columnInfo.Type == "String" {
				if columnInfo.IsArray {
					cellDataList := strings.Split(cellData, "/")
					if serverField.IsValid() {
						serverField.Set(reflect.ValueOf(cellDataList))
					}
					if clientField.IsValid() {
						clientField.Set(reflect.ValueOf(cellDataList))
					}
				} else {
					if serverField.IsValid() {
						serverField.SetString(cellData)
					}
					if clientField.IsValid() {
						clientField.SetString(cellData)
					}
				}
			} else if columnInfo.Type == "Float" {
				if columnInfo.IsArray {
					cellDataList := strings.Split(cellData, "/")
					var sliceData []float64
					for _, data := range cellDataList {
						val, err := strconv.ParseFloat(data, 64)
						HandleErr(err)

						sliceData = append(sliceData, val)
					}

					if serverField.IsValid() {
						serverField.Set(reflect.ValueOf(sliceData))
					}
					if clientField.IsValid() {
						clientField.Set(reflect.ValueOf(sliceData))
					}
				} else {
					val, err := strconv.ParseFloat(cellData, 64)
					HandleErr(err)
					if serverField.IsValid() {
						serverField.SetFloat(val)
					}
					if clientField.IsValid() {
						clientField.SetFloat(val)
					}
				}
			} else if columnInfo.Type == "Enum" {
				enumColData := EnumData[columnInfo.TypeValue]
				if columnInfo.IsArray {
					cellDataList := strings.Split(cellData, "/")
					var sliceData []int
					for _, data := range cellDataList {
						enumVal, ok := enumColData[data]
						if !ok {
							ThrowErr(EnumErr.ERR_REF_ENUM, tableName)
						}
						sliceData = append(sliceData, enumVal)
					}

					if serverField.IsValid() {
						serverField.Set(reflect.ValueOf(sliceData))
					}
					if clientField.IsValid() {
						clientField.Set(reflect.ValueOf(sliceData))
					}
				} else {
					enumVal, ok := enumColData[cellData]
					if !ok {
						ThrowErr(EnumErr.ERR_REF_ENUM, tableName)
					}

					if serverField.IsValid() {
						serverField.SetInt(int64(enumVal))
					}
					if clientField.IsValid() {
						clientField.SetInt(int64(enumVal))
					}
				}
			} else if columnInfo.Type == "Table" {
				tableRowData := TableData[columnInfo.TypeValue]

				if columnInfo.IsArray {
					cellDataList := strings.Split(cellData, "/")
					var sliceData []int
					for _, data := range cellDataList {
						tableVal, ok := tableRowData[data]
						if !ok {
							ThrowErr(EnumErr.ERR_REF_TABLE, tableName)
						}
						sliceData = append(sliceData, tableVal)
					}

					if serverField.IsValid() {
						serverField.Set(reflect.ValueOf(sliceData))
					}
					if clientField.IsValid() {
						clientField.Set(reflect.ValueOf(sliceData))
					}
				} else {
					tableVal, ok := tableRowData[cellData]
					if !ok {
						ThrowErr(EnumErr.ERR_REF_TABLE, tableName)
					}
					if serverField.IsValid() {
						serverField.SetInt(int64(tableVal))
					}
					if clientField.IsValid() {
						clientField.SetInt(int64(tableVal))
					}
				}
			}
		}
		serverData = append(serverData, rowServerData.Interface())
		clientData = append(clientData, rowClientData.Interface())
	}

	tableName = ExtractTableName(tableName)

	sj, err := json.Marshal(serverData)
	HandleErr(err)
	err = ioutil.WriteFile("./Server/"+tableName+".json", sj, os.FileMode(0644))
	HandleErr(err)

	cj, err := json.Marshal(clientData)
	HandleErr(err)
	err = ioutil.WriteFile("./Client/"+tableName+".json", cj, os.FileMode(0644))
	HandleErr(err)

	fmt.Println("Extract Finish <" + tableName + ">")
}

// Enum 파일 생성
func CreateEnumFile() {
	var strs []string
	strs = append(strs,
		"package main\n\n",
	)

	// 변수 추가
	for _, key := range EnumList {
		strs = append(strs,
			"type ",
			key,
			"Value int\n",
		)
	}

	strs = append(strs,
		"\n",
		"const (\n",
	)

	isFirst := true
	for _, key := range EnumList {
		if isFirst {
			isFirst = false
		} else {
			strs = append(strs, "\n")
		}
		enumColData := EnumData[key]
		enumDataList := SortEnumData(enumColData)
		count := 0
		for _, enumKey := range enumDataList {
			strs = append(strs,
				"\t",
				key,
				"_",
				enumKey,
				"\t=\t",
				key,
				"Value(",
				strconv.Itoa(count),
				")\n",
			)
			count++
		}
	}
	strs = append(strs, ")\n\n")

	err := ioutil.WriteFile("./Server/DesignEnums.go", []byte(strings.Join(strs, "")), os.FileMode(0644))
	HandleErr(err)
}

// 구조체 파일 생성
func CreateTableFile() {
	var strs []string
	strs = append(strs,
		"package main\n\n",
		"import (\n",
		"\t\"encoding/json\"\n",
		"\t\"io/ioutil\"\n",
		")\n\n",
	)

	strs = append(strs,
		"var (\n",
	)
	for _, tableName := range TableList {
		tableName = ExtractTableName(tableName)
		strs = append(strs,
			"\t",
			tableName,
			"Data map[int]",
			tableName,
			"\n",
		)
	}
	strs = append(strs,
		")\n\n",
	)

	for _, tableName := range TableList {
		columnInfoMap := TableStruct[tableName]
		columnInfoList := SortTableColumn(columnInfoMap)

		tableName = ExtractTableName(tableName)

		strs = append(strs,
			"type ",
			tableName,
			" struct {\n",
		)
		for _, columnInfo := range columnInfoList {
			if columnInfo.UsePlace == "Client" || columnInfo.UsePlace == "None" {
				continue
			}

			strs = append(strs,
				"\t",
				columnInfo.Name,
				"\t",
			)
			if columnInfo.IsArray {
				strs = append(strs,
					"[]",
				)
			}

			if columnInfo.Type == "Int" || columnInfo.Type == "Enum" || columnInfo.Type == "Table" {
				strs = append(strs, "int\n")
			} else if columnInfo.Type == "String\n" {
				strs = append(strs, "string")
			} else if columnInfo.Type == "Float" {
				strs = append(strs, "float64\n")
			}
		}
		strs = append(strs,
			"}\n\n",
		)
	}

	// 데이터 unmarshal 함수
	strs = append(strs,
		"func InitDesignTables() {\n",
		"\tvar bytes []byte\n",
		"\tvar err error\n\n",
	)

	for _, tableName := range TableList {
		tableName = ExtractTableName(tableName)
		strs = append(strs,
			"\t",
			tableName,
			"Data = make(map[int]",
			tableName,
			")\n\tvar ",
			tableName,
			"Val []",
			tableName,
			"\n",
		)
	}

	strs = append(strs, "\n")

	for _, tableName := range TableList {
		tableName = ExtractTableName(tableName)
		strs = append(strs,
			"\tbytes, err = ioutil.ReadFile(\"./Table/",
			tableName,
			".json\")\n",
			"\tHandleErr(err)\n",
			"\terr = json.Unmarshal(bytes, &",
			tableName,
			"Val)\n",
			"\tHandleErr(err)\n",
		)
	}

	strs = append(strs, "\n")

	for _, tableName := range TableList {
		tableName = ExtractTableName(tableName)
		strs = append(strs,
			"\tfor _, data := range ",
			tableName,
			"Val {\n",
			"\t\t",
			tableName,
			"Data[data.Key] = data\n",
			"\t}\n",
		)
	}
	strs = append(strs,
		"}\n\n",
	)

	err := ioutil.WriteFile("./Server/DesignTables.go", []byte(strings.Join(strs, "")), os.FileMode(0644))
	HandleErr(err)
}

func InitTables() {

}

type Test_Table01 struct {
	Key    int
	Test02 float64
	Test04 int
	Test06 int
}

// 테이블 구조체 생성
func CreateStruct(columnInfoMap map[int]ColumnInfo) (serverStruct, clientStruct reflect.Type) {
	var serverFields []reflect.StructField
	var clientFields []reflect.StructField

	columnInfoList := SortTableColumn(columnInfoMap)
	for _, columnInfo := range columnInfoList {
		var columnType reflect.Type
		if columnInfo.Type == "Int" || columnInfo.Type == "Enum" || columnInfo.Type == "Table" {
			if columnInfo.IsArray {
				columnType = reflect.TypeOf([]int{})
			} else {
				columnType = reflect.TypeOf(int(0))
			}
		} else if columnInfo.Type == "Float" {
			if columnInfo.IsArray {
				columnType = reflect.TypeOf([]float64{})
			} else {
				columnType = reflect.TypeOf(float64(0))
			}
		} else {
			if columnInfo.IsArray {
				columnType = reflect.TypeOf([]string{})
			} else {
				columnType = reflect.TypeOf(string(""))
			}
		}

		structField := reflect.StructField{
			Name: columnInfo.Name,
			Type: columnType,
		}
		if columnInfo.UsePlace == "All" {
			serverFields = append(serverFields, structField)
			clientFields = append(clientFields, structField)
		} else if columnInfo.UsePlace == "Server" {
			serverFields = append(serverFields, structField)
		} else if columnInfo.UsePlace == "Client" {
			clientFields = append(clientFields, structField)
		}
	}

	serverStruct = reflect.StructOf(serverFields)
	clientStruct = reflect.StructOf(clientFields)
	return
}

func SortEnumData(enumColData map[string]int) []string {
	result := make([]string, len(enumColData))
	for enumKey, enumValue := range enumColData {
		result[enumValue] = enumKey
	}
	return result
}

func SortTableColumn(columnInfoMap map[int]ColumnInfo) []ColumnInfo {
	result := make([]ColumnInfo, len(columnInfoMap))
	for index, columnInfo := range columnInfoMap {
		result[index] = columnInfo
	}
	return result
}

func ExtractTableName(tableName string) string {
	splitStr := strings.Split(tableName, ".")
	if len(splitStr) == 1 {
		return splitStr[0]
	} else {
		return splitStr[1]
	}
}

// func LoopObjectField(object reflect.Value) {
// 	for i := 0; i < object.NumField(); i++ {
// 		v := object.Field(i)
// 		t := object.Type().Field(i)
// 		fmt.Printf("Name: %s / Type: %s / Value: %v / Tag: %s \n",
// 			t.Name, t.Type, v.Interface(), t.Tag.Get("custom"))
// 	}
// }
