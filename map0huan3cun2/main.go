package main

import (
	"bufio"
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"io"
	"os"
	"strings"
	"time"
)

func ShowErr(err error, where string) {
	if err != nil {
		fmt.Println("报错", where, err)
	}
}
func ExitErr(err error, where string) {
	if err != nil {
		fmt.Println("报错", where, err)
		os.Exit(1)
	}
}

//先将数据初始化写进数据库中，后续再运行程序因tag.txt这一标记文件的存在而不再操作该步骤
func init() {
	fileInfo, err := os.Stat("./tag.txt")
	if err == nil {
		fmt.Println("无需初始化数据入库，标记文件已存在：", fileInfo.Name())
		return
	}
	//fmt.Println("环境变量=",os.Environ())
	mydb, err := sql.Open("mysql", "root:413188ok@tcp(localhost:3306)/kai1fang2")
	ExitErr(err, "15行")
	defer mydb.Close()
	//建表
	_, err = mydb.Exec("create table if not exists kfinfo(id int auto_increment primary key," +
		"name varchar(15) not null," +
		"idcard char(18) not null," +
		"hotel varchar(15) not null);") //分号、括号不要掉，也可直接将建表的sql语句写在``内
	ExitErr(err, "25行")

	//将文件中数据写入数据库
	kffile, err := os.Open("./kf.txt")
	ExitErr(err, "31行")
	defer kffile.Close()
	reader := bufio.NewReader(kffile)
	for {
		lineByte, _, err := reader.ReadLine()
		if err == io.EOF {
			//todo:创建一个标记文件,即使程序关了，该标记依然在，便于再次启动程序不用初始化数据入库
			os.Create("./tag.txt")
			fmt.Println("数据库数据初始化完成，创建了标记空文件tag.txt")
			break
		}
		ShowErr(err, "37行")
		lineSlice := strings.Split(string(lineByte), ",")
		name := strings.TrimSpace(lineSlice[0])
		idcard := strings.TrimSpace(lineSlice[1])
		hotel := strings.TrimSpace(lineSlice[2])
		_, err = mydb.Exec("insert into kfinfo(name,idcard,hotel)values(?,?,?)", name, idcard, hotel)
		if err != nil {
			fmt.Println("插入该条数据失败：", string(lineByte))
		}
	}
}

var db *sql.DB

type PersonInfo struct {
	Id     int
	Name   string
	Idcard string
	Hotel  string
}
type PersonsInfos struct {
	PersonInfoSlice *[]PersonInfo
	lookUptime      int64 //标记该条开房记录最新一次被查询的时间
	lookUpCount     int   //标记该条开房记录被查询的次数
}

var guestMap = make(map[string]*PersonsInfos) //键值对的键存储的是客户端每次查询输入的内容
func main() {

	db, err := sql.Open("mysql", "root:413188ok@tcp(localhost:3306)/kai1fang2")
	ExitErr(err, "15行")
	defer db.Close()
	//客户端查询
	var lookUp string
	var guest PersonInfo

	for {
		fmt.Print("请输入查询内容：")
		fmt.Scanln(&lookUp) //输入查询对象，可输入  %红  作为模糊查询
		start := time.Now()
		//先从缓存中查   //todo:据视频老师说，内存中map查询是很快的
		if personsInfos := guestMap[lookUp]; personsInfos != nil { //todo:map进行判断，在缓存中有人查过lookup对应的数据
			fmt.Println("查询结果是：", personsInfos.PersonInfoSlice)
			personsInfos.lookUptime = time.Now().UnixNano() //每被查一次，其被查时间就会更新成最新的，
			personsInfos.lookUpCount++
			fmt.Println("缓存中查询耗时：", time.Now().Sub(start))
			continue
		}

		//不行再到数据库中查
		rows, err := db.Query("select * from kfinfo where name like ?", lookUp)
		if err != nil {
			fmt.Println("查询失败")
			continue
		}
		//todo:上面这里如果是写完整的模糊查询尾号是1的用户的sql语句，则需是select * from kfinfo where name like '%1' 也就是
		//todo:需要有单引号，上面由于lookUp类型就是string，所以没事。而且在cmd中输入时不加 %或_就会是精确查询
		PersonInfoSlice := []PersonInfo{}
		for rows.Next() {
			rows.Scan(&guest.Id, &guest.Name, &guest.Idcard, &guest.Hotel) //todo:这里不能直接row.Scan(&guest)，需要逐个字段赋值
			PersonInfoSlice = append(PersonInfoSlice, guest)
		}
		fmt.Println("查询结果是:", PersonInfoSlice)
		fmt.Println("数据库查询耗时：", time.Now().Sub(start))

		//todo:添加进内存中作为缓存
		personsInfos := PersonsInfos{
			PersonInfoSlice: &PersonInfoSlice,
			lookUptime:      time.Now().UnixNano(),
		}
		personsInfos.lookUpCount++
		guestMap[lookUp] = &personsInfos

		//对缓存进行老旧数据的清理   //todo:---------可以做成一个可以被广泛使用的缓存清理小框架函数
		if len(guestMap) > 10 {
			early := time.Now().UnixNano()
			var key string
			for k, v := range guestMap {
				if v.lookUptime < early {
					key = k
					early = v.lookUptime
				}
			}
			delete(guestMap, key) //删掉最久远的那一条数据,其实也可以用（当前时间 - 被查时间）/v.lookUpCount 来综合确定最该被删的缓存
		}
	}
}
