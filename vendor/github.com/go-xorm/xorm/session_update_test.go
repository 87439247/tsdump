// Copyright 2017 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package xorm

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/go-xorm/core"
	"github.com/stretchr/testify/assert"
)

func TestUpdateMap(t *testing.T) {
	assert.NoError(t, prepareEngine())

	type UpdateTable struct {
		Id   int64
		Name string
		Age  int
	}

	assert.NoError(t, testEngine.Sync2(new(UpdateTable)))
	var tb = UpdateTable{
		Name: "test",
		Age:  35,
	}
	_, err := testEngine.Insert(&tb)
	assert.NoError(t, err)

	cnt, err := testEngine.Table("update_table").Where("id = ?", tb.Id).Update(map[string]interface{}{
		"name": "test2",
		"age":  36,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)
}

func TestUpdateLimit(t *testing.T) {
	assert.NoError(t, prepareEngine())

	type UpdateTable struct {
		Id   int64
		Name string
		Age  int
	}

	assert.NoError(t, testEngine.Sync2(new(UpdateTable)))
	var tb = UpdateTable{
		Name: "test1",
		Age:  35,
	}
	cnt, err := testEngine.Insert(&tb)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	tb.Name = "test2"
	tb.Id = 0
	cnt, err = testEngine.Insert(&tb)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	cnt, err = testEngine.OrderBy("name desc").Limit(1).Update(&UpdateTable{
		Age: 30,
	})
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	var uts []UpdateTable
	err = testEngine.Find(&uts)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, len(uts))
	assert.EqualValues(t, 35, uts[0].Age)
	assert.EqualValues(t, 30, uts[1].Age)
}

type ForUpdate struct {
	Id   int64 `xorm:"pk"`
	Name string
}

func setupForUpdate(engine *Engine) error {
	v := new(ForUpdate)
	err := testEngine.DropTables(v)
	if err != nil {
		return err
	}
	err = testEngine.CreateTables(v)
	if err != nil {
		return err
	}

	list := []ForUpdate{
		{1, "data1"},
		{2, "data2"},
		{3, "data3"},
	}

	for _, f := range list {
		_, err = testEngine.Insert(f)
		if err != nil {
			return err
		}
	}
	return nil
}

func TestForUpdate(t *testing.T) {
	if testEngine.DriverName() != "mysql" && testEngine.DriverName() != "mymysql" {
		return
	}

	err := setupForUpdate(testEngine)
	if err != nil {
		t.Error(err)
		return
	}

	session1 := testEngine.NewSession()
	session2 := testEngine.NewSession()
	session3 := testEngine.NewSession()
	defer session1.Close()
	defer session2.Close()
	defer session3.Close()

	// start transaction
	err = session1.Begin()
	if err != nil {
		t.Error(err)
		return
	}

	// use lock
	fList := make([]ForUpdate, 0)
	session1.ForUpdate()
	session1.Where("(id) = ?", 1)
	err = session1.Find(&fList)
	switch {
	case err != nil:
		t.Error(err)
		return
	case len(fList) != 1:
		t.Errorf("find not returned single row")
		return
	case fList[0].Name != "data1":
		t.Errorf("for_update.name must be `data1`")
		return
	}

	// wait for lock
	wg := &sync.WaitGroup{}

	// lock is used
	wg.Add(1)
	go func() {
		f2 := new(ForUpdate)
		session2.Where("(id) = ?", 1).ForUpdate()
		has, err := session2.Get(f2) // wait release lock
		switch {
		case err != nil:
			t.Error(err)
		case !has:
			t.Errorf("cannot find target row. for_update.id = 1")
		case f2.Name != "updated by session1":
			t.Errorf("read lock failed")
		}
		wg.Done()
	}()

	// lock is NOT used
	wg.Add(1)
	go func() {
		f3 := new(ForUpdate)
		session3.Where("(id) = ?", 1)
		has, err := session3.Get(f3) // wait release lock
		switch {
		case err != nil:
			t.Error(err)
		case !has:
			t.Errorf("cannot find target row. for_update.id = 1")
		case f3.Name != "data1":
			t.Errorf("read lock failed")
		}
		wg.Done()
	}()

	// wait for go rountines
	time.Sleep(50 * time.Millisecond)

	f := new(ForUpdate)
	f.Name = "updated by session1"
	session1.Where("(id) = ?", 1)
	session1.Update(f)

	// release lock
	err = session1.Commit()
	if err != nil {
		t.Error(err)
		return
	}

	wg.Wait()
}

func TestWithIn(t *testing.T) {
	type temp3 struct {
		Id   int64  `xorm:"Id pk autoincr"`
		Name string `xorm:"Name"`
		Test bool   `xorm:"Test"`
	}

	assert.NoError(t, prepareEngine())
	assert.NoError(t, testEngine.Sync(new(temp3)))

	testEngine.Insert(&[]temp3{
		{
			Name: "user1",
		},
		{
			Name: "user1",
		},
		{
			Name: "user1",
		},
	})

	cnt, err := testEngine.In("Id", 1, 2, 3, 4).Update(&temp3{Name: "aa"}, &temp3{Name: "user1"})
	assert.NoError(t, err)
	assert.EqualValues(t, 3, cnt)
}

type Condi map[string]interface{}

type UpdateAllCols struct {
	Id     int64
	Bool   bool
	String string
	Ptr    *string
}

type UpdateMustCols struct {
	Id     int64
	Bool   bool
	String string
}

type UpdateIncr struct {
	Id   int64
	Cnt  int
	Name string
}

type Article struct {
	Id      int32  `xorm:"pk INT autoincr"`
	Name    string `xorm:"VARCHAR(45)"`
	Img     string `xorm:"VARCHAR(100)"`
	Aside   string `xorm:"VARCHAR(200)"`
	Desc    string `xorm:"VARCHAR(200)"`
	Content string `xorm:"TEXT"`
	Status  int8   `xorm:"TINYINT(4)"`
}

func TestUpdateMap2(t *testing.T) {
	assert.NoError(t, prepareEngine())
	assertSync(t, new(UpdateMustCols))

	_, err := testEngine.Table("update_must_cols").Where("id =?", 1).Update(map[string]interface{}{
		"bool": true,
	})
	if err != nil {
		t.Error(err)
		panic(err)
	}
}

func TestUpdate1(t *testing.T) {
	assert.NoError(t, prepareEngine())
	assertSync(t, new(Userinfo))

	_, err := testEngine.Insert(&Userinfo{
		Username: "user1",
	})

	var ori Userinfo
	has, err := testEngine.Get(&ori)
	if err != nil {
		t.Error(err)
		panic(err)
	}
	if !has {
		t.Error(errors.New("not exist"))
		panic(errors.New("not exist"))
	}

	// update by id
	user := Userinfo{Username: "xxx", Height: 1.2}
	cnt, err := testEngine.Id(ori.Uid).Update(&user)
	if err != nil {
		t.Error(err)
		panic(err)
	}
	if cnt != 1 {
		err = errors.New("update not returned 1")
		t.Error(err)
		panic(err)
		return
	}

	condi := Condi{"username": "zzz", "departname": ""}
	cnt, err = testEngine.Table(&user).Id(ori.Uid).Update(&condi)
	if err != nil {
		t.Error(err)
		panic(err)
	}
	if cnt != 1 {
		err = errors.New("update not returned 1")
		t.Error(err)
		panic(err)
		return
	}

	cnt, err = testEngine.Update(&Userinfo{Username: "yyy"}, &user)
	if err != nil {
		t.Error(err)
		panic(err)
	}
	total, err := testEngine.Count(&user)
	if err != nil {
		t.Error(err)
		panic(err)
	}

	if cnt != total {
		err = errors.New("insert not returned 1")
		t.Error(err)
		panic(err)
		return
	}

	// nullable update
	{
		user := &Userinfo{Username: "not null data", Height: 180.5}
		_, err := testEngine.Insert(user)
		if err != nil {
			t.Error(err)
			panic(err)
		}
		userID := user.Uid

		has, err := testEngine.Id(userID).
			And("username = ?", user.Username).
			And("height = ?", user.Height).
			And("departname = ?", "").
			And("detail_id = ?", 0).
			And("is_man = ?", 0).
			Get(&Userinfo{})
		if err != nil {
			t.Error(err)
			panic(err)
		}
		if !has {
			err = errors.New("cannot insert properly")
			t.Error(err)
			panic(err)
		}

		updatedUser := &Userinfo{Username: "null data"}
		cnt, err = testEngine.Id(userID).
			Nullable("height", "departname", "is_man", "created").
			Update(updatedUser)
		if err != nil {
			t.Error(err)
			panic(err)
		}
		if cnt != 1 {
			err = errors.New("update not returned 1")
			t.Error(err)
			panic(err)
		}

		has, err = testEngine.Id(userID).
			And("username = ?", updatedUser.Username).
			And("height IS NULL").
			And("departname IS NULL").
			And("is_man IS NULL").
			And("created IS NULL").
			And("detail_id = ?", 0).
			Get(&Userinfo{})
		if err != nil {
			t.Error(err)
			panic(err)
		}
		if !has {
			err = errors.New("cannot update with null properly")
			t.Error(err)
			panic(err)
		}

		cnt, err = testEngine.Id(userID).Delete(&Userinfo{})
		if err != nil {
			t.Error(err)
			panic(err)
		}
		if cnt != 1 {
			err = errors.New("delete not returned 1")
			t.Error(err)
			panic(err)
		}
	}

	err = testEngine.StoreEngine("Innodb").Sync2(&Article{})
	if err != nil {
		t.Error(err)
		panic(err)
	}

	defer func() {
		err = testEngine.DropTables(&Article{})
		if err != nil {
			t.Error(err)
			panic(err)
		}
	}()

	a := &Article{0, "1", "2", "3", "4", "5", 2}
	cnt, err = testEngine.Insert(a)
	if err != nil {
		t.Error(err)
		panic(err)
	}

	if cnt != 1 {
		err = errors.New(fmt.Sprintf("insert not returned 1 but %d", cnt))
		t.Error(err)
		panic(err)
	}

	if a.Id == 0 {
		err = errors.New("insert returned id is 0")
		t.Error(err)
		panic(err)
	}

	cnt, err = testEngine.Id(a.Id).Update(&Article{Name: "6"})
	if err != nil {
		t.Error(err)
		panic(err)
	}

	if cnt != 1 {
		err = errors.New(fmt.Sprintf("insert not returned 1 but %d", cnt))
		t.Error(err)
		panic(err)
		return
	}

	var s = "test"

	col1 := &UpdateAllCols{Ptr: &s}
	err = testEngine.Sync(col1)
	if err != nil {
		t.Error(err)
		panic(err)
	}

	_, err = testEngine.Insert(col1)
	if err != nil {
		t.Error(err)
		panic(err)
	}

	col2 := &UpdateAllCols{col1.Id, true, "", nil}
	_, err = testEngine.Id(col2.Id).AllCols().Update(col2)
	if err != nil {
		t.Error(err)
		panic(err)
	}

	col3 := &UpdateAllCols{}
	has, err = testEngine.Id(col2.Id).Get(col3)
	if err != nil {
		t.Error(err)
		panic(err)
	}

	if !has {
		err = errors.New(fmt.Sprintf("cannot get id %d", col2.Id))
		t.Error(err)
		panic(err)
		return
	}

	if *col2 != *col3 {
		err = errors.New(fmt.Sprintf("col2 should eq col3"))
		t.Error(err)
		panic(err)
		return
	}

	{

		col1 := &UpdateMustCols{}
		err = testEngine.Sync(col1)
		if err != nil {
			t.Error(err)
			panic(err)
		}

		_, err = testEngine.Insert(col1)
		if err != nil {
			t.Error(err)
			panic(err)
		}

		col2 := &UpdateMustCols{col1.Id, true, ""}
		boolStr := testEngine.ColumnMapper.Obj2Table("Bool")
		stringStr := testEngine.ColumnMapper.Obj2Table("String")
		_, err = testEngine.Id(col2.Id).MustCols(boolStr, stringStr).Update(col2)
		if err != nil {
			t.Error(err)
			panic(err)
		}

		col3 := &UpdateMustCols{}
		has, err := testEngine.Id(col2.Id).Get(col3)
		if err != nil {
			t.Error(err)
			panic(err)
		}

		if !has {
			err = errors.New(fmt.Sprintf("cannot get id %d", col2.Id))
			t.Error(err)
			panic(err)
			return
		}

		if *col2 != *col3 {
			err = errors.New(fmt.Sprintf("col2 should eq col3"))
			t.Error(err)
			panic(err)
			return
		}
	}
}

func TestUpdateIncrDecr(t *testing.T) {
	assert.NoError(t, prepareEngine())

	col1 := &UpdateIncr{
		Name: "test",
	}
	assert.NoError(t, testEngine.Sync(col1))

	_, err := testEngine.Insert(col1)
	assert.NoError(t, err)

	colName := testEngine.ColumnMapper.Obj2Table("Cnt")

	cnt, err := testEngine.Id(col1.Id).Incr(colName).Update(col1)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	newCol := new(UpdateIncr)
	has, err := testEngine.Id(col1.Id).Get(newCol)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, 1, newCol.Cnt)

	cnt, err = testEngine.Id(col1.Id).Decr(colName).Update(col1)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)

	newCol = new(UpdateIncr)
	has, err = testEngine.Id(col1.Id).Get(newCol)
	assert.NoError(t, err)
	assert.True(t, has)
	assert.EqualValues(t, 0, newCol.Cnt)

	cnt, err = testEngine.Id(col1.Id).Cols(colName).Incr(colName).Update(col1)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, cnt)
}

type UpdatedUpdate struct {
	Id      int64
	Updated time.Time `xorm:"updated"`
}

type UpdatedUpdate2 struct {
	Id      int64
	Updated int64 `xorm:"updated"`
}

type UpdatedUpdate3 struct {
	Id      int64
	Updated int `xorm:"updated bigint"`
}

type UpdatedUpdate4 struct {
	Id      int64
	Updated int `xorm:"updated"`
}

type UpdatedUpdate5 struct {
	Id      int64
	Updated time.Time `xorm:"updated bigint"`
}

func TestUpdateUpdated(t *testing.T) {
	assert.NoError(t, prepareEngine())

	di := new(UpdatedUpdate)
	err := testEngine.Sync2(di)
	if err != nil {
		t.Fatal(err)
	}

	_, err = testEngine.Insert(&UpdatedUpdate{})
	if err != nil {
		t.Fatal(err)
	}

	ci := &UpdatedUpdate{}
	_, err = testEngine.Id(1).Update(ci)
	if err != nil {
		t.Fatal(err)
	}

	has, err := testEngine.Id(1).Get(di)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal(ErrNotExist)
	}
	if ci.Updated.Unix() != di.Updated.Unix() {
		t.Fatal("should equal:", ci, di)
	}
	fmt.Println("ci:", ci, "di:", di)

	di2 := new(UpdatedUpdate2)
	err = testEngine.Sync2(di2)
	if err != nil {
		t.Fatal(err)
	}

	_, err = testEngine.Insert(&UpdatedUpdate2{})
	if err != nil {
		t.Fatal(err)
	}
	ci2 := &UpdatedUpdate2{}
	_, err = testEngine.Id(1).Update(ci2)
	if err != nil {
		t.Fatal(err)
	}
	has, err = testEngine.Id(1).Get(di2)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal(ErrNotExist)
	}
	if ci2.Updated != di2.Updated {
		t.Fatal("should equal:", ci2, di2)
	}
	fmt.Println("ci2:", ci2, "di2:", di2)

	di3 := new(UpdatedUpdate3)
	err = testEngine.Sync2(di3)
	if err != nil {
		t.Fatal(err)
	}

	_, err = testEngine.Insert(&UpdatedUpdate3{})
	if err != nil {
		t.Fatal(err)
	}
	ci3 := &UpdatedUpdate3{}
	_, err = testEngine.Id(1).Update(ci3)
	if err != nil {
		t.Fatal(err)
	}

	has, err = testEngine.Id(1).Get(di3)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal(ErrNotExist)
	}
	if ci3.Updated != di3.Updated {
		t.Fatal("should equal:", ci3, di3)
	}
	fmt.Println("ci3:", ci3, "di3:", di3)

	di4 := new(UpdatedUpdate4)
	err = testEngine.Sync2(di4)
	if err != nil {
		t.Fatal(err)
	}

	_, err = testEngine.Insert(&UpdatedUpdate4{})
	if err != nil {
		t.Fatal(err)
	}

	ci4 := &UpdatedUpdate4{}
	_, err = testEngine.Id(1).Update(ci4)
	if err != nil {
		t.Fatal(err)
	}

	has, err = testEngine.Id(1).Get(di4)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal(ErrNotExist)
	}
	if ci4.Updated != di4.Updated {
		t.Fatal("should equal:", ci4, di4)
	}
	fmt.Println("ci4:", ci4, "di4:", di4)

	di5 := new(UpdatedUpdate5)
	err = testEngine.Sync2(di5)
	if err != nil {
		t.Fatal(err)
	}

	_, err = testEngine.Insert(&UpdatedUpdate5{})
	if err != nil {
		t.Fatal(err)
	}
	ci5 := &UpdatedUpdate5{}
	_, err = testEngine.Id(1).Update(ci5)
	if err != nil {
		t.Fatal(err)
	}

	has, err = testEngine.Id(1).Get(di5)
	if err != nil {
		t.Fatal(err)
	}
	if !has {
		t.Fatal(ErrNotExist)
	}
	if ci5.Updated.Unix() != di5.Updated.Unix() {
		t.Fatal("should equal:", ci5, di5)
	}
	fmt.Println("ci5:", ci5, "di5:", di5)
}

func TestUpdateSameMapper(t *testing.T) {
	assert.NoError(t, prepareEngine())

	oldMapper := testEngine.ColumnMapper
	testEngine.unMapType(rValue(new(Userinfo)).Type())
	testEngine.unMapType(rValue(new(Condi)).Type())
	testEngine.unMapType(rValue(new(Article)).Type())
	testEngine.unMapType(rValue(new(UpdateAllCols)).Type())
	testEngine.unMapType(rValue(new(UpdateMustCols)).Type())
	testEngine.unMapType(rValue(new(UpdateIncr)).Type())
	testEngine.SetMapper(core.SameMapper{})
	defer func() {
		testEngine.unMapType(rValue(new(Userinfo)).Type())
		testEngine.unMapType(rValue(new(Condi)).Type())
		testEngine.unMapType(rValue(new(Article)).Type())
		testEngine.unMapType(rValue(new(UpdateAllCols)).Type())
		testEngine.unMapType(rValue(new(UpdateMustCols)).Type())
		testEngine.unMapType(rValue(new(UpdateIncr)).Type())
		testEngine.SetMapper(oldMapper)
	}()

	assertSync(t, new(Userinfo))

	_, err := testEngine.Insert(&Userinfo{
		Username: "user1",
	})
	assert.NoError(t, err)

	var ori Userinfo
	has, err := testEngine.Get(&ori)
	if err != nil {
		t.Error(err)
		panic(err)
	}
	if !has {
		t.Error(errors.New("not exist"))
		panic(errors.New("not exist"))
	}
	// update by id
	user := Userinfo{Username: "xxx", Height: 1.2}
	cnt, err := testEngine.Id(ori.Uid).Update(&user)
	if err != nil {
		t.Error(err)
		panic(err)
	}
	if cnt != 1 {
		err = errors.New("update not returned 1")
		t.Error(err)
		panic(err)
		return
	}

	condi := Condi{"Username": "zzz", "Departname": ""}
	cnt, err = testEngine.Table(&user).Id(ori.Uid).Update(&condi)
	if err != nil {
		t.Error(err)
		panic(err)
	}

	if cnt != 1 {
		err = errors.New("update not returned 1")
		t.Error(err)
		panic(err)
		return
	}

	cnt, err = testEngine.Update(&Userinfo{Username: "yyy"}, &user)
	if err != nil {
		t.Error(err)
		panic(err)
	}

	total, err := testEngine.Count(&user)
	if err != nil {
		t.Error(err)
		panic(err)
	}

	if cnt != total {
		err = errors.New("insert not returned 1")
		t.Error(err)
		panic(err)
		return
	}

	err = testEngine.Sync(&Article{})
	if err != nil {
		t.Error(err)
		panic(err)
	}

	defer func() {
		err = testEngine.DropTables(&Article{})
		if err != nil {
			t.Error(err)
			panic(err)
		}
	}()

	a := &Article{0, "1", "2", "3", "4", "5", 2}
	cnt, err = testEngine.Insert(a)
	if err != nil {
		t.Error(err)
		panic(err)
	}

	if cnt != 1 {
		err = errors.New(fmt.Sprintf("insert not returned 1 but %d", cnt))
		t.Error(err)
		panic(err)
	}

	if a.Id == 0 {
		err = errors.New("insert returned id is 0")
		t.Error(err)
		panic(err)
	}

	cnt, err = testEngine.Id(a.Id).Update(&Article{Name: "6"})
	if err != nil {
		t.Error(err)
		panic(err)
	}

	if cnt != 1 {
		err = errors.New(fmt.Sprintf("insert not returned 1 but %d", cnt))
		t.Error(err)
		panic(err)
		return
	}

	col1 := &UpdateAllCols{}
	err = testEngine.Sync(col1)
	if err != nil {
		t.Error(err)
		panic(err)
	}

	_, err = testEngine.Insert(col1)
	if err != nil {
		t.Error(err)
		panic(err)
	}

	col2 := &UpdateAllCols{col1.Id, true, "", nil}
	_, err = testEngine.Id(col2.Id).AllCols().Update(col2)
	if err != nil {
		t.Error(err)
		panic(err)
	}

	col3 := &UpdateAllCols{}
	has, err = testEngine.Id(col2.Id).Get(col3)
	if err != nil {
		t.Error(err)
		panic(err)
	}

	if !has {
		err = errors.New(fmt.Sprintf("cannot get id %d", col2.Id))
		t.Error(err)
		panic(err)
		return
	}

	if *col2 != *col3 {
		err = errors.New(fmt.Sprintf("col2 should eq col3"))
		t.Error(err)
		panic(err)
		return
	}

	{
		col1 := &UpdateMustCols{}
		err = testEngine.Sync(col1)
		if err != nil {
			t.Error(err)
			panic(err)
		}

		_, err = testEngine.Insert(col1)
		if err != nil {
			t.Error(err)
			panic(err)
		}

		col2 := &UpdateMustCols{col1.Id, true, ""}
		boolStr := testEngine.ColumnMapper.Obj2Table("Bool")
		stringStr := testEngine.ColumnMapper.Obj2Table("String")
		_, err = testEngine.Id(col2.Id).MustCols(boolStr, stringStr).Update(col2)
		if err != nil {
			t.Error(err)
			panic(err)
		}

		col3 := &UpdateMustCols{}
		has, err := testEngine.Id(col2.Id).Get(col3)
		if err != nil {
			t.Error(err)
			panic(err)
		}

		if !has {
			err = errors.New(fmt.Sprintf("cannot get id %d", col2.Id))
			t.Error(err)
			panic(err)
			return
		}

		if *col2 != *col3 {
			err = errors.New(fmt.Sprintf("col2 should eq col3"))
			t.Error(err)
			panic(err)
			return
		}
	}

	{

		col1 := &UpdateIncr{}
		err = testEngine.Sync(col1)
		if err != nil {
			t.Error(err)
			panic(err)
		}

		_, err = testEngine.Insert(col1)
		if err != nil {
			t.Error(err)
			panic(err)
		}

		cnt, err := testEngine.Id(col1.Id).Incr("`Cnt`").Update(col1)
		if err != nil {
			t.Error(err)
			panic(err)
		}
		if cnt != 1 {
			err = errors.New("update incr failed")
			t.Error(err)
			panic(err)
		}

		newCol := new(UpdateIncr)
		has, err := testEngine.Id(col1.Id).Get(newCol)
		if err != nil {
			t.Error(err)
			panic(err)
		}
		if !has {
			err = errors.New("has incr failed")
			t.Error(err)
			panic(err)
		}
		if 1 != newCol.Cnt {
			err = errors.New("incr failed")
			t.Error(err)
			panic(err)
		}
	}
}

func TestUseBool(t *testing.T) {
	assert.NoError(t, prepareEngine())
	assertSync(t, new(Userinfo))

	cnt1, err := testEngine.Count(&Userinfo{})
	if err != nil {
		t.Error(err)
		panic(err)
	}

	users := make([]Userinfo, 0)
	err = testEngine.Find(&users)
	if err != nil {
		t.Error(err)
		panic(err)
	}
	var fNumber int64
	for _, u := range users {
		if u.IsMan == false {
			fNumber += 1
		}
	}

	cnt2, err := testEngine.UseBool().Update(&Userinfo{IsMan: true})
	if err != nil {
		t.Error(err)
		panic(err)
	}
	if fNumber != cnt2 {
		fmt.Println("cnt1", cnt1, "fNumber", fNumber, "cnt2", cnt2)
		/*err = errors.New("Updated number is not corrected.")
		  t.Error(err)
		  panic(err)*/
	}

	_, err = testEngine.Update(&Userinfo{IsMan: true})
	if err == nil {
		err = errors.New("error condition")
		t.Error(err)
		panic(err)
	}
}

func TestBool(t *testing.T) {
	assert.NoError(t, prepareEngine())
	assertSync(t, new(Userinfo))

	_, err := testEngine.UseBool().Update(&Userinfo{IsMan: true})
	if err != nil {
		t.Error(err)
		panic(err)
	}
	users := make([]Userinfo, 0)
	err = testEngine.Find(&users)
	if err != nil {
		t.Error(err)
		panic(err)
	}
	for _, user := range users {
		if !user.IsMan {
			err = errors.New("update bool or find bool error")
			t.Error(err)
			panic(err)
		}
	}

	_, err = testEngine.UseBool().Update(&Userinfo{IsMan: false})
	if err != nil {
		t.Error(err)
		panic(err)
	}
	users = make([]Userinfo, 0)
	err = testEngine.Find(&users)
	if err != nil {
		t.Error(err)
		panic(err)
	}
	for _, user := range users {
		if user.IsMan {
			err = errors.New("update bool or find bool error")
			t.Error(err)
			panic(err)
		}
	}
}

func TestNoUpdate(t *testing.T) {
	assert.NoError(t, prepareEngine())

	type NoUpdate struct {
		Id      int64
		Content string
	}

	assertSync(t, new(NoUpdate))

	_, err := testEngine.Insert(&NoUpdate{
		Content: "test",
	})
	assert.NoError(t, err)

	_, err = testEngine.Id(1).Update(&NoUpdate{})
	assert.Error(t, err)
	assert.EqualValues(t, "No content found to be updated", err.Error())
}

func TestNewUpdate(t *testing.T) {
	assert.NoError(t, prepareEngine())

	type TbUserInfo struct {
		Id       int64       `xorm:"pk autoincr unique BIGINT" json:"id"`
		Phone    string      `xorm:"not null unique VARCHAR(20)" json:"phone"`
		UserName string      `xorm:"VARCHAR(20)" json:"user_name"`
		Gender   int         `xorm:"default 0 INTEGER" json:"gender"`
		Pw       string      `xorm:"VARCHAR(100)" json:"pw"`
		Token    string      `xorm:"TEXT" json:"token"`
		Avatar   string      `xorm:"TEXT" json:"avatar"`
		Extras   interface{} `xorm:"JSON" json:"extras"`
		Created  time.Time   `xorm:"DATETIME created"`
		Updated  time.Time   `xorm:"DATETIME updated"`
		Deleted  time.Time   `xorm:"DATETIME deleted"`
	}

	assertSync(t, new(TbUserInfo))

	targetUsr := TbUserInfo{Phone: "13126564922"}
	changeUsr := TbUserInfo{Token: "ABCDEFG"}
	af, err := testEngine.Update(&changeUsr, &targetUsr)
	assert.NoError(t, err)
	assert.EqualValues(t, 0, af)

	af, err = testEngine.Table(new(TbUserInfo)).Where("phone=?", 13126564922).Update(&changeUsr)
	assert.NoError(t, err)
	assert.EqualValues(t, 0, af)
}
