package db

import (
  "log"
  "time"
  //"database/sql"
  "github.com/jmoiron/sqlx"
  _ "github.com/go-sql-driver/mysql"
  "alertstrap/config"
)

var (
  Conn *sqlx.DB
  Conf config.Config
)

type Alert struct {
  Mess_id    string     `json:"mess_id" db:"mess_id"`
  Mgrp_id    string     `json:"mgrp_id" db:"mgrp_id"`
  Host       string     `json:"host" db:"host"`
  Severity   string     `json:"severity" db:"severity"`
  Sv_level   int        `json:"sv_level" db:"sv_level"`
  Ts_min     time.Time  `json:"sv_level" db:"ts_min"`
  Ts_max     time.Time  `json:"ts_max" db:"ts_max"`
  Ts_unix    int32      `json:"ts_unix" db:"ts_unix"`
  Text       string     `json:"text" db:"text"`
  Duplicate  int        `json:"duplicate" db:"duplicate"`
  Port_id    int        `json:"port_id" db:"port_id"`
  Appl_id    int        `json:"appl_id" db:"appl_id"`
  Instance   string     `json:"instance" db:"instance"`
  Mib        string     `json:"mib" db:"mib"`
  Param      string     `json:"param" db:"param"`
  Object     string     `json:"object" db:"object"`
  Short_oid  string     `json:"short_oid" db:"short_oid"`
  Full_oid   string     `json:"full_oid" db:"full_oid"`
  Zone       string     `json:"zone" db:"zone"`
  Stand      string     `json:"stand" db:"stand"`
  Place      string     `json:"place" db:"place"`
  Url        string     `json:"url" db:"url"`
  Trap_usr_1 string     `json:"trap_usr_1" db:"trap_usr_1"`
  Trap_usr_2 string     `json:"trap_usr_2" db:"trap_usr_2"`
  Trap_usr_3 string     `json:"trap_usr_3" db:"trap_usr_3"`
  Trap_usr_4 string     `json:"trap_usr_4" db:"trap_usr_4"`
  Trap_usr_5 string     `json:"trap_usr_5" db:"trap_usr_5"`
}

func ConnectDb(cfg config.Config) (*sqlx.DB) {
  Conf = cfg
  db, err := sqlx.Connect("mysql", cfg.Mysql.Conn_string)
  if err != nil {
    log.Printf("[error] v%", err)
    return nil
  }
  return db
}

func AddAlert(alrt *Alert) {
  tx := Conn.MustBegin()
  _, err := tx.NamedExec(`
    insert into `+Conf.Mysql.Alerts_table+`
      (
        mess_id, mgrp_id, host, severity, sv_level, ts_unix, text,
        port_id, appl_id, instance, mib, param, object, short_oid, full_oid, stand, url,
        trap_usr_1, trap_usr_2, trap_usr_3, trap_usr_4, trap_usr_5
      )
    values
      (
        :mess_id, :mgrp_id, :host, :severity, :sv_level, :ts_unix, :text,
        :port_id, :appl_id, :instance, :mib, :param, :object, :short_oid, :full_oid, :stand, :url,
        :trap_usr_1, :trap_usr_2, :trap_usr_3, :trap_usr_4, :trap_usr_5
      )
  `, alrt)
  if err != nil {
    log.Printf("[error] v%", err)
    return
  }
  tx.Commit()
  return
}

func UpdAlert(alrt *Alert) {
  tx := Conn.MustBegin()
  _, err := tx.NamedExec(`
    update `+Conf.Mysql.Alerts_table+`
    set text=:text, duplicate=:duplicate, ts_max=now(), ts_unix=:ts_unix
  `, alrt)
  if err != nil {
    log.Printf("[error] v%", err)
    return
  }
  tx.Commit()
  return
}

func LoadAlerts() ([]*Alert) {
  rows, err := Conn.Queryx("select * from "+Conf.Mysql.Alerts_view)
  if err != nil {
    log.Printf("[error] v%", err)
    return nil
  }

  var alts []*Alert
  for rows.Next() {
    var a *Alert
    err = rows.StructScan(&a)
    alts = append(alts, a)
  }

  return alts
}

func CreateSchema() {
  Conn.MustExec(`
    create table if not exists `+Conf.Mysql.Alerts_table+` (
      mess_id varchar(50) not null,
      mgrp_id varchar(50) not null,
      host varchar(100) not null,
      real_host varchar(100),
      severity varchar(10) not null,
      sv_level int default 0,
      ts_min datetime default now(),
      ts_max datetime default now(),
      ts_unix bigint default 0,
      text text,
      duplicate int default 1,
      port_id int,
      appl_id varchar(10),
      instance varchar(50),
      mib varchar (250),
      param varchar (255),
      object varchar (350),
      short_oid varchar (200),
      full_oid varchar (1500),
      stand varchar (50),
      zone varchar (50),
      place varchar (50),
      url varchar(500),
      trap_usr_1 varchar(500),
      trap_usr_2 varchar(500),
      trap_usr_3 varchar(500),
      trap_usr_4 varchar(500),
      trap_usr_5 varchar(500),
      primary key IDX_mon_alerts_mess_id (mess_id),
      key IDX_mon_alerts_mgrp_id (mgrp_id),
      key IDX_mon_alerts_ts_max (ts_max),
      key IDX_mon_alerts_mgrp_id_ts_max (mgrp_id,ts_max)
    ) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci
  `)
  Conn.MustExec(`
    create or replace view `+Conf.Mysql.Alerts_view+` as
    select
      mes.mess_id, mes.mgrp_id, mes.host, mes.severity, mes.sv_level,
      date_format(convert_tz(mes.ts_min, '+00:00', '-03:00'), '%Y-%m-%dT%H:%i:%sZ') as ts_min,
      date_format(convert_tz(mes.ts_max, '+00:00', '-03:00'), '%Y-%m-%dT%H:%i:%sZ') as ts_max,
      mes.ts_unix, mes.text, mes.duplicate,
      mes.port_id, mes.appl_id, mes.instance, mes.mib, mes.param, mes.object,
      mes.short_oid, mes.full_oid, mes.stand, mes.zone, mes.place, mes.url,
      mes.trap_usr_1, mes.trap_usr_2, mes.trap_usr_3, mes.trap_usr_4, mes.trap_usr_5
    from `+Conf.Mysql.Alerts_table+` as mes
    inner join (
      select mgrp_id, max(ts_max) as ts_max from `+Conf.Mysql.Alerts_table+`
      where ts_max > now() - interval 30 minute group by mgrp_id
    ) as mes2 on mes.mgrp_id = mes2.mgrp_id and mes.ts_max = mes2.ts_max
    where mes.ts_max > now() - interval 30 minute
  `)
}
