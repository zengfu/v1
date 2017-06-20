package main

import (
	"fmt"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/zengfu/v1/broker"
	"time"
)

func main() {
	db, err := gorm.Open("mysql", "root:71451085Zf*@/test?charset=utf8&parseTime=True&loc=Local")
	if err != nil {
		fmt.Println(err)
	}
	defer db.Close()
	db.LogMode(true)

	//db.Model(&User{}).AddForeignKey("profile_id", "profiles(id)", "RESTRICT", "RESTRICT")

	db.AutoMigrate(&broker.Client{})
	db.AutoMigrate(&broker.Message{})
	db.AutoMigrate(&broker.Subscribe{})

	//db.Exec(" alter table subscriptions add constraint sub unique (session_id,topic);")
	//db.Where("session_id=?", "test").Delete(Session{})
	//var ts Session
	//fmt.Println(db.Where("session_id=?", "test").Delete(Session{})

	//db.Model(&s).Related(&subs, "Subscribes")

	fmt.Println(time.Now())
}
