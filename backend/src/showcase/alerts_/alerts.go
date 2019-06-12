package alerts

import (
  "crypto/sha1"
  "encoding/base64"
  "time"
  "strings"
  "showcase/db"
)

//type Alert struct {
//  Mess_id    string  `json:"mess_id"`
//  Mgrp_id    string  `json:"mgrp_id"`
//  Host       string  `json:"host"`
//  Real_host  string  `json:"real_host"`
//  Severity   string  `json:"severity"`
//  Sv_level   int     `json:"sv_level"`
//  Ts_min     string  `json:"ts_min"`
//  Ts_max     string  `json:"ts_max"`
//  Ts_unix    int32   `json:"ts_unix"`
//  Text       string  `json:"text"`
//  Duplicate  int     `json:"duplicate"`
//  Port_id    int     `json:"port_id"`
//  Appl_id    int     `json:"appl_id"`
//  Instance   string  `json:"instance"`
//  Mib        string  `json:"mib"`
//  Param      string  `json:"param"`
//  Object     string  `json:"object"`
//  Short_oid  string  `json:"short_oid"`
//  Full_oid   string  `json:"full_oid"`
//  Zone       string  `json:"zone"`
//  Stand      string  `json:"stand"`
//  Trap_usr_1 string  `json:"trap_usr_1"`
//  Trap_usr_2 string  `json:"trap_usr_2"`
//  Trap_usr_3 string  `json:"trap_usr_3"`
//  Trap_usr_4 string  `json:"trap_usr_4"`
//  Trap_usr_5 string  `json:"trap_usr_5"`
//}

var (
  Alerts = make(map[string](*db.Alert))
)

func GetAlerts() (map[string](*db.Alert)) {
  return Alerts
}

func DelAlert(id string) {
  delete(Alerts, id)
  return
}

func GetHash(bv []byte) (string) {
  hasher := sha1.New()
  hasher.Write(bv)
  return base64.URLEncoding.EncodeToString(hasher.Sum(nil))
}

func GetSvLevel(lv string) (int) {
  return 5
}

func AddAlert(dat *db.Alert) (){
  dat.Ts_unix  = int32(time.Now().Unix())
  dat.Severity = strings.ToLower(dat.Severity)
  dat.Sv_level = GetSvLevel(dat.Severity)
  dat.Mgrp_id  = GetHash([]byte(dat.Host+dat.Param+dat.Instance+dat.Object))
  dat.Mess_id  = GetHash([]byte(string(dat.Ts_unix)+dat.Mgrp_id+dat.Severity))

  if alrt, ok := Alerts[dat.Mgrp_id]; ok {
    alrt.Duplicate += 1
    alrt.Text = dat.Text
    alrt.Severity = dat.Severity
    alrt.Sv_level = dat.Sv_level
    alrt.Zone = dat.Zone
    alrt.Stand = dat.Stand
    alrt.Ts_unix = dat.Ts_unix
    db.AddAlert(alrt)
  } else {
    Alerts[dat.Mgrp_id] = dat
    Alerts[dat.Mgrp_id].Duplicate = 1
  }

  return
}
