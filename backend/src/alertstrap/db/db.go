package db

import (
  "os"
  "fmt"
  "log"
  "strings"
  "strconv"
  "io/ioutil"
  "database/sql"
  _ "github.com/go-sql-driver/mysql"
  "alertstrap/config"
)

var (
  db    *sql.DB
  cfg   config.Config
)

func ConnectDb(conf config.Config) error {
  cfg = conf
  if db == nil {
    conn, err := sql.Open("mysql", cfg.Mysql.Conn_string)
    if err != nil {
      return err
    }
    db = conn
  }
  return nil
}

func getResult(sel string, exec []interface{}) ([]map[string]interface{}, error) {
  result := []map[string]interface{}{}

  rows, err := db.Query(sel, exec...)
  if err != nil {
    return nil, err
  }
  defer rows.Close()

  columns, err := rows.ColumnTypes()
  if err != nil {
    return nil, err
  }

  // Make a slice for the values
	values := make([]sql.RawBytes, len(columns))

	scanArgs := make([]interface{}, len(values))
	for i := range values {
		scanArgs[i] = &values[i]
	}

  for rows.Next() {

    if err := rows.Scan(scanArgs...); err != nil {
      continue
		}

    alert := make(map[string]interface{})
    for i, value := range values {
      switch columns[i].DatabaseTypeName() {
        case "INT":
          cl, _ := strconv.Atoi(string(value))
          alert[columns[i].Name()] = int(cl)
        case "BIGINT":
          cl, _ := strconv.Atoi(string(value))
          alert[columns[i].Name()] = int64(cl)
      	default:
      		alert[columns[i].Name()] = string(value)
    	}
    }
    result = append(result, alert)
  }

  return result, nil
}

func AddAlert(alrt map[string]interface{}) error {

  keys := make([]string, 0, len(alrt))
  vals := make([]string, 0, len(alrt))
  exec := []interface{}{}
  for k := range alrt {
    keys = append(keys, k)
    vals = append(vals, "?")
    exec = append(exec, alrt[k])
  }

  stmt, err := db.Prepare(fmt.Sprintf("insert into %s (%s) values (%s)", cfg.Mysql.Alerts_table, strings.Join(keys, ", "), strings.Join(vals, ", ")))
	if err != nil {
    return err
	}
  defer stmt.Close()

  _, err = stmt.Exec(exec...)
  if err != nil {
    return err
	}

  return nil
}

func UpdAlert(alrt map[string]interface{}, mess_id string) error {

  vals := make([]string, 0, len(alrt))
  exec := []interface{}{}
  for k := range alrt {
    vals = append(vals, k+" = ?")
    exec = append(exec, alrt[k])
  }
  exec = append(exec, mess_id)

  stmt, err := db.Prepare(fmt.Sprintf("update %s set %s where mess_id = ?", cfg.Mysql.Alerts_table, strings.Join(vals, ", ")))
	if err != nil {
    return err
	}
  defer stmt.Close()

  _, err = stmt.Exec(exec...)
  if err != nil {
    return err
	}

  return nil
}

func LoadHosts() ([]map[string]interface{}, error) {
  return getResult(fmt.Sprintf("select * from %s", cfg.Mysql.Hosts_view), nil)
}

func LoadAlerts() ([]map[string]interface{}, error) {
  return getResult(fmt.Sprintf("select * from %s", cfg.Mysql.Alerts_view), nil)
}

func GetHistory(query map[string][]string) ([]map[string]interface{}, error) {
  vals := make([]string, 0, len(query))
  vals = append(vals, "ts_max > NOW() - interval 30 day")
  exec := []interface{}{}
  for k, v := range query {
    vals = append(vals, k+" like ?")
    exec = append(exec, v[0])
  }

  return getResult(fmt.Sprintf("select * from %s where %s order by ts_max desc", cfg.Mysql.Alerts_table, strings.Join(vals, " and ")), exec)
}

func UpdateTask(cfg config.Config, mgrp_id string, task_key string) bool {
  key := GetTaskKey(cfg, mgrp_id)
  if key != "" {
    err := db.QueryRow("update "+cfg.Mysql.Tasks_table+" set task_key=? where mgrp_id=?", task_key, mgrp_id)
  	if err != nil {
      log.Printf("[error] %v", err)
  		return false
  	}
  } else {
    err := db.QueryRow("insert into "+cfg.Mysql.Tasks_table+" (mgrp_id, task_key) values (?, ?)", mgrp_id, task_key)
  	if err != nil {
      log.Printf("[error] %v", err)
  		return false
  	}
  }
  return true
}

func GetTaskKey(cfg config.Config, mgrp_id string) string {
  var task_key string
  err := db.QueryRow("select task_key from "+cfg.Mysql.Tasks_table+" where mgrp_id = ?", mgrp_id).Scan(&task_key)
  if err != nil {
    log.Printf("[error] %v", err)
    return ""
  }
  return task_key
}

func CreateSchema(filename string) error {
  fl, err := os.Open(filename)
  if err != nil {
    return err
  }
  defer fl.Close()

  tx, err := ioutil.ReadAll(fl)
  if err != nil {
    return err
  }

  _, err = db.Query(string(tx))
	if err != nil {
    return err
	}

  return nil
}
