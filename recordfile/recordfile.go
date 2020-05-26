// 数据记录成文件
package recordfile

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
)

// 字段分割符
var Comma = '\t'
// 注释符
var Comment = '#'

// 索引 - 空的接口映射
type Index map[interface{}]interface{}

// 记录文件数据结构
type RecordFile struct {
	Comma      rune             // 字段分隔符
	Comment    rune             // 注释符
	typeRecord reflect.Type     // 自己的反射类型
	records    []interface{}    // 记录数据
	indexes    []Index          // 索引数据
}

// 根据传入的数据类型生成一个记录文件对象
func New(st interface{}) (*RecordFile, error) {
	// 获取数据类型的反射类型
	typeRecord := reflect.TypeOf(st)
	// 只支持结构体类型数据
	if typeRecord == nil || typeRecord.Kind() != reflect.Struct {
		return nil, errors.New("st must be a struct")
	}

	// 循环存储数据类型的字段
	for i := 0; i < typeRecord.NumField(); i++ {
		// 获取类型字段
		f := typeRecord.Field(i)

		// 取类型字段的kind
		kind := f.Type.Kind()
		switch kind {
		case reflect.Bool:     // 布尔型
		case reflect.Int:      // 整数型
		case reflect.Int8:     // 8位整数型
		case reflect.Int16:    // 16位整数型
		case reflect.Int32:    // 32位整数型
		case reflect.Int64:    // 64位整数型
		case reflect.Uint:     // 无符号整数型
		case reflect.Uint8:    // 8位无符号整数型  
		case reflect.Uint16:   // 16位无符号整数型
		case reflect.Uint32:   // 32位无符号整数型
		case reflect.Uint64:   // 64位无符号整数型
		case reflect.Float32:  // 32位浮点型
		case reflect.Float64:  // 64位浮点型
		case reflect.String:   // 字符串类型
		case reflect.Struct:   // 结构体
		case reflect.Array:    // 数组
		case reflect.Slice:    // 切片
		case reflect.Map:      // 映射
		default:
			return nil, fmt.Errorf("invalid type: %v %s",
				f.Name, kind)
		}

		// 获取字段的标签，索引字段类型判断
		tag := f.Tag
		if tag == "index" {
			// 索引字段只支持基础数据类型
			switch kind {
			case reflect.Struct, reflect.Slice, reflect.Map:
				return nil, fmt.Errorf("could not index %s field %v %v",
					kind, i, f.Name)
			}
		}
	}

	// 创建新的记录文件数据
	rf := new(RecordFile)
	// 记录自己的反射类型
	rf.typeRecord = typeRecord

	return rf, nil
}

// 从文件读取数据到记录文件
func (rf *RecordFile) Read(name string) error {
	// 打开文件
	file, err := os.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()

	// 初始行分割符
	if rf.Comma == 0 {
		rf.Comma = Comma
	}
	// 初始注释符
	if rf.Comment == 0 {
		rf.Comment = Comment
	}
	// 初始化csv读取器
	reader := csv.NewReader(file)
	reader.Comma = rf.Comma
	reader.Comment = rf.Comment
	// 读取所有行数据，数据存储到二维的数据切片中
	lines, err := reader.ReadAll()
	if err != nil {
		return err
	}

	// 获取反射类型
	typeRecord := rf.typeRecord

	// 使用空接口切片创建记录数据
	// make records
	records := make([]interface{}, len(lines)-1)

	// 使用新的切片索引创建索引集合
	// make indexes
	indexes := []Index{}
	// 循环记录字段
	for i := 0; i < typeRecord.NumField(); i++ {
		// 获取字段标签
		tag := typeRecord.Field(i).Tag
		if tag == "index" {
			// 有index的字段都创建一个索引集合
			indexes = append(indexes, make(Index))
		}
	}

	// 循环读取各行数据
	for n := 1; n < len(lines); n++ {
		// 从反射类型创建一个新的反射零值
		value := reflect.New(typeRecord)
		// 零值的行数据通过类型断言转化为接口型，并存储到数据记录中
		records[n-1] = value.Interface()
		// 零值记录数据
		record := value.Elem()
		
		// 一行记录数据
		line := lines[n]
		if len(line) != typeRecord.NumField() {
			// 字段数量和一行数据数量 统一
			return fmt.Errorf("line %v, field count mismatch: %v (file) %v (st)",
				n, len(line), typeRecord.NumField())
		}

		iIndex := 0

		// 循环字段
		for i := 0; i < typeRecord.NumField(); i++ {
			// 获取字段
			f := typeRecord.Field(i)

			// 数据记录
			// records
			strField := line[i]
			// 字段
			field := record.Field(i)
			if !field.CanSet() {
				// 判断字段是否可修改
				continue
			}

			var err error

			// 获取字段类型的Kind
			kind := f.Type.Kind()
			if kind == reflect.Bool {
				// bool型
				var v bool
				v, err = strconv.ParseBool(strField)
				if err == nil {
					field.SetBool(v)
				}
			} else if kind == reflect.Int ||
				kind == reflect.Int8 ||
				kind == reflect.Int16 ||
				kind == reflect.Int32 ||
				kind == reflect.Int64 {
				// 整数类型
				var v int64
				v, err = strconv.ParseInt(strField, 0, f.Type.Bits())
				if err == nil {
					field.SetInt(v)
				}
			} else if kind == reflect.Uint ||
				kind == reflect.Uint8 ||
				kind == reflect.Uint16 ||
				kind == reflect.Uint32 ||
				kind == reflect.Uint64 {
				// 无符号整数类型
				var v uint64
				v, err = strconv.ParseUint(strField, 0, f.Type.Bits())
				if err == nil {
					field.SetUint(v)
				}
			} else if kind == reflect.Float32 ||
				kind == reflect.Float64 {
				// 浮点型
				var v float64
				v, err = strconv.ParseFloat(strField, f.Type.Bits())
				if err == nil {
					field.SetFloat(v)
				}
			} else if kind == reflect.String {
				// 字符串型
				field.SetString(strField)
			} else if kind == reflect.Struct ||
				kind == reflect.Array ||
				kind == reflect.Slice ||
				kind == reflect.Map {
				// 结构体、数组、切片、映射 都采用JSON反序列化
				err = json.Unmarshal([]byte(strField), field.Addr().Interface())
			}

			if err != nil {
				return fmt.Errorf("parse field (row=%v, col=%v) error: %v",
					n, i, err)
			}

			// 字段索引处理
			// indexes
			if f.Tag == "index" {
				// 从索引集合获取索引
				index := indexes[iIndex]
				// 索引总数自增
				iIndex++
				// 字段当前值是否已在索引中（索引字段的值不能重复）
				if _, ok := index[field.Interface()]; ok {
					return fmt.Errorf("index error: duplicate at (row=%v, col=%v)",
						n, i)
				}
				// 索引当前值对应记录（切片）
				index[field.Interface()] = records[n-1]
			}
		}
	}

	rf.records = records
	rf.indexes = indexes

	return nil
}

// 获取某条记录
func (rf *RecordFile) Record(i int) interface{} {
	return rf.records[i]
}

// 记录文件的条数
func (rf *RecordFile) NumRecord() int {
	return len(rf.records)
}

// 获取某条索引
func (rf *RecordFile) Indexes(i int) Index {
	if i >= len(rf.indexes) {
		return nil
	}
	return rf.indexes[i]
}

// 获取索引
func (rf *RecordFile) Index(i interface{}) interface{} {
	index := rf.Indexes(0)
	if index == nil {
		return nil
	}
	return index[i]
}
