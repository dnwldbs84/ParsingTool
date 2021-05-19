package main

import (
	"fmt"
	"os"
	"sync"
)

var (
	TableList   []string
	EnumData    map[string]map[string]int
	TableData   map[string]map[string]int
	TableStruct map[string]map[int]ColumnInfo

	EnumList []string

	TableLock sync.RWMutex

	EnumErr EnumErrStruct
)

type EnumErrStruct struct {
	ERR_SYSTEM    EnumInfo
	ERR_ENUM      EnumInfo
	ERR_KEY       EnumInfo
	ERR_NAME      EnumInfo
	ERR_TYPE      EnumInfo
	ERR_USE_PLACE EnumInfo
	ERR_REF_ENUM  EnumInfo
	ERR_REF_TABLE EnumInfo
}

type ColumnInfo struct {
	// Num      int // column number
	Name      string
	Type      string // int, string, float, Enum, Table
	TypeValue string // EnumName, TableName
	UsePlace  string // server, client, all, none
	IsArray   bool
}

type EnumInfo struct {
	Key   string
	Value int
}

func Initialize() {
	// 폴더 체크
	if _, err := os.Stat("./Server"); os.IsNotExist(err) {
		err = os.Mkdir("./Server", os.FileMode(0644))
		HandleErr(err)
	}
	if _, err := os.Stat("./Client"); os.IsNotExist(err) {
		err = os.Mkdir("./Client", os.FileMode(0644))
		HandleErr(err)
	}

	// 변수 초기화
	EnumData = make(map[string]map[string]int)
	TableData = make(map[string]map[string]int)
	TableStruct = make(map[string]map[int]ColumnInfo)

	// Enum 초기화
	EnumErr.ERR_SYSTEM = EnumInfo{"ERR_SYSTEM", 0}
	EnumErr.ERR_ENUM = EnumInfo{"ERR_ENUM", 1}
	EnumErr.ERR_KEY = EnumInfo{"ERR_KEY", 2}
	EnumErr.ERR_NAME = EnumInfo{"ERR_NAME", 3}
	EnumErr.ERR_TYPE = EnumInfo{"ERR_TYPE", 4}
	EnumErr.ERR_USE_PLACE = EnumInfo{"ERR_USE_PLACE", 5}
	EnumErr.ERR_REF_ENUM = EnumInfo{"ERR_REF_ENUM", 6}
	EnumErr.ERR_REF_TABLE = EnumInfo{"ERR_REF_TABLE", 7}
}

func HandleErr(err error) {
	if err != nil {
		fmt.Println(err.Error())
		ThrowErr(EnumErr.ERR_SYSTEM, "")
	}
}

func ThrowErr(errEnum EnumInfo, tableName string) {
	panic(errEnum.Key + " [" + tableName + "]")
}
