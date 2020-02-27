package db

import (
  //"os"
  //"fmt"
  "log"
  //"time"
  //"strings"
  "strconv"
  //"io/ioutil"
  "encoding/json"
  "database/sql"
  _ "github.com/go-sql-driver/mysql"
  "alertstrap/config"
  "alertstrap/cache"
)

var (
  db *sql.DB
)

func ConnectDb(conf config.Config) error {
  if db == nil {
    conn, err := sql.Open("mysql", conf.Mysql.Conn_string)
    if err != nil {
      return err
    }
    db = conn
  }
  return nil
}

func LoadAlerts() ([]cache.Alert, error) {
  result := []cache.Alert{}

  rows, err := db.Query("select * from mon_v_alerts")
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
    var a cache.Alert

    if err := rows.Scan(scanArgs...); err != nil {
      continue
		}

    for i, value := range values {
      switch columns[i].Name() {
        case "alert_id":
          a.AlertId = string(value)
        case "group_id":
          a.GroupId = string(value)
        case "status":
          a.Status = string(value)
        case "starts_at":
          cl, err := strconv.Atoi(string(value))
          if err == nil {
            a.StartsAt = int64(cl)
          }
        case "ends_at":
          cl, err := strconv.Atoi(string(value))
          if err == nil {
            a.EndsAt = int64(cl)
          }
        case "duplicate":
          cl, err := strconv.Atoi(string(value))
          if err == nil {
            a.Duplicate = int(cl)
          }
        case "labels":
          if err := json.Unmarshal(value, &a.Labels); err != nil {
            log.Printf("[warning] %v (%s)", err, a.AlertId)
          }
        case "annotations":
          if err := json.Unmarshal(value, &a.Annotations); err != nil {
            log.Printf("[warning] %v (%s)", err, a.AlertId)
          }
      }
    }

    result = append(result, a) 
  }

  return result, nil
}

func SaveItems(items map[string]cache.Item) {

  stmt, err := db.Prepare("replace into mon_alerts values (?,?,?,?,?,?,?,?,?)")
  if err != nil {
    log.Printf("[error] %v", err)
    return
	}
  defer stmt.Close()

  for _, i := range items {

    labels, err := json.Marshal(i.Value.Labels)
    if err != nil {
      log.Printf("[error] %v", err)
      continue
    }

    annotations, err := json.Marshal(i.Value.Annotations)
    if err != nil {
      log.Printf("[error] %v", err)
      continue
    }

    _, err = stmt.Exec(
      i.Value.AlertId,
      i.Value.GroupId,
      i.Value.Status,
      i.Value.StartsAt,
      i.Value.EndsAt,
      i.Value.Duplicate,
      labels,
      annotations,
      i.Value.GeneratorURL,
    )
    if err != nil {
      log.Printf("[error] %v", err)
      continue
    }

    log.Printf("[info] alert recorded in database - %s", i.Value.AlertId)
  }

}

func AddAlert(alert cache.Alert) error {

  stmt, err := db.Prepare("insert into mon_alerts values (?,?,?,?,?,?,?,?,?)")
	if err != nil {
    return err
	}
  defer stmt.Close()

  labels, err := json.Marshal(alert.Labels)
  if err != nil {
    return err
  }

  annotations, err := json.Marshal(alert.Annotations)
  if err != nil {
    return err
  }

  _, err = stmt.Exec(
    alert.AlertId,
    alert.GroupId,
    alert.Status,
    alert.StartsAt,
    alert.EndsAt,
    alert.Duplicate,
    labels,
    annotations,
    alert.GeneratorURL,
  )
  if err != nil {
    return err
	}

  return nil
}

func UpdAlert(alert cache.Alert) error {

  stmt, err := db.Prepare("update mon_alerts set status=?,ends_at=?,duplicate=?,annotations=?,generator_url=? where alert_id = ?")
	if err != nil {
    return err
	}
  defer stmt.Close()

  annotations, err := json.Marshal(alert.Annotations)
  if err != nil {
    return err
  }

  _, err = stmt.Exec(
    alert.Status,
    alert.EndsAt,
    alert.Duplicate,
    annotations,
    alert.GeneratorURL,
    alert.AlertId,
  )
  if err != nil {
    return err
	}

  return nil
}

/*

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

*/

/*

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
  //return getResult(fmt.Sprintf("select * from %s", cfg.Mysql.Hosts_view), nil)
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

*/
