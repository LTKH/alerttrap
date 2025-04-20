package mysql

import (
    "log"
    "time"
    "fmt"
    "strconv"
    "encoding/json"
    "database/sql"
    msl "github.com/go-sql-driver/mysql"
    "github.com/ltkh/alerttrap/internal/config"
    "github.com/ltkh/alerttrap/internal/cache"
)

type Client struct {
    client *sql.DB
    config *config.DB
}

func NewClient(conf *config.DB) (*Client, error) {
    if conf.ConnString == "" {
        cfg := msl.Config{
            User:                 conf.User,
            Passwd:               conf.Password,
            Net:                  "tcp",
            Addr:                 conf.Host,
            DBName:               conf.Name,
            AllowNativePasswords: true,
        }
        conf.ConnString = cfg.FormatDSN()
    }

    conn, err := sql.Open("mysql", conf.ConnString)
    if err != nil {
        return nil, err
    }

    return &Client{ client: conn, config: conf }, nil
}

func (db *Client) Close() error {
	db.client.Close()

	return nil
}

func (db *Client) CreateTables() error {
    _, err := db.client.Exec(fmt.Sprintf(`
      create table if not exists alerts (
        %[1]salert_id%[1]s      varchar(50) not null,
        %[1]sgroup_id%[1]s      varchar(50) not null,
        %[1]sstate%[1]s         varchar(10) not null,
        %[1]sactive_at%[1]s     bigint(20) default 0,
        %[1]sstarts_at%[1]s     bigint(20) default 0,
        %[1]sends_at%[1]s       bigint(20) default 0,
        %[1]srepeat%[1]s        int default 1,
        %[1]schange_st%[1]s     int default 0,
        %[1]slabels%[1]s        json,
        %[1]sannotations%[1]s   json,
        %[1]sgenerator_url%[1]s varchar(1500),
        unique key IDX_mon_alerts_alert_id (alert_id),
        key IDX_mon_alerts_ends_at (ends_at),
        key IDX_mon_alerts_group_id_ends_at (group_id,ends_at)
      ) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci
    `, "`"))
    if err != nil {
        return err
    }

    _, err = db.client.Exec(fmt.Sprintf(`
      create table if not exists users (
        %[1]slogin%[1]s         varchar(100) not null,
        %[1]spassword%[1]s      varchar(100) not null,
        %[1]sname%[1]s          varchar(150),
        %[1]semail%[1]s         varchar(100),
        %[1]stoken%[1]s         varchar(100) not null,
        unique key IDX_mon_users_login (login)
      ) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci
    `, "`"))

    _, err = db.client.Exec(fmt.Sprintf(`
      create table if not exists proxy_logs (
        %[1]sid%[1]s            int not null auto_increment,
        %[1]slogin%[1]s         varchar(100) not null,
        %[1]smethod%[1]s        varchar(10),
        %[1]surl%[1]s           varchar(500),
        %[1]spath%[1]s          varchar(1000),
        %[1]stimestamp%[1]s     bigint(20) default 0,
        primary key (id)
      ) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci
    `, "`"))
    if err != nil {
        return err
    }

    _, err = db.client.Exec(fmt.Sprintf(`
      create table if not exists actions (
        %[1]sid%[1]s            int not null auto_increment,
        %[1]slogin%[1]s         varchar(100) not null,
        %[1]saction%[1]s        varchar(100),
        %[1]sobject%[1]s        varchar(100),
        %[1]sattributes%[1]s    json,
        %[1]sdescription%[1]s   text,
        %[1]stimestamp%[1]s     bigint(20) default 0,
        primary key (id)
      ) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci
    `, "`"))
    if err != nil {
        return err
    }

    return nil
}

func (db *Client) Healthy() error {
    stmt, err := db.client.Prepare("select alert_id from alerts a where a.ends_at > UNIX_TIMESTAMP() limit 1")
    if err != nil {
        return err
    }
    defer stmt.Close()

    return nil
}

func (db *Client) LoadUser(login string) (cache.User, error) {
    var usr cache.User

    stmt, err := db.client.Prepare("select login,name,password,token from users where login = ?")
    if err != nil {
        return usr, err
    }
    defer stmt.Close()

    err = stmt.QueryRow(login).Scan(&usr.Login, &usr.Name, &usr.Password, &usr.Token)
    if err != nil {
        return usr, err
    }

    return usr, nil
}

func (db *Client) SaveUser(user cache.User) error {
    stmt, err := db.client.Prepare("replace into users (login,name,password,token) values (?,?,?,?)")
    if err != nil {
        return err
    }
    defer stmt.Close()

    _, err = stmt.Exec(user.Login, user.Name, user.Password, user.Token)
    if err != nil {
        return err
    }

    return nil
}

func (db *Client) LoadUsers() ([]cache.User, error) {
    result := []cache.User{}

    rows, err := db.client.Query("select login,password,token from users")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    for rows.Next() {
        var usr cache.User
        usr.EndsAt = time.Now().UTC().Unix()
        err := rows.Scan(&usr.Login, &usr.Password, &usr.Token)
        if err != nil {
            return nil, err
        }
        result = append(result, usr) 
    }

    return result, nil
}

func (db *Client) LoadAlerts() ([]cache.Alert, error) {
    result := []cache.Alert{}

    rows, err := db.client.Query("select * from alerts a where a.ends_at > UNIX_TIMESTAMP() and a.ends_at = (select max(ends_at) from alerts where group_id = a.group_id)")
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
                case "state":
                    a.State = string(value)
                case "active_at":
                    cl, err := strconv.Atoi(string(value))
                    if err == nil {
                        a.ActiveAt = int64(cl)
                    }
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
                case "repeat":
                    cl, err := strconv.Atoi(string(value))
                    if err == nil {
                        a.Repeat = int(cl)
                    }
                case "change_st":
                    cl, err := strconv.Atoi(string(value))
                    if err == nil {
                        a.ChangeSt = int(cl)
                    }
                case "labels":
                    if err := json.Unmarshal(value, &a.Labels); err != nil {
                        log.Printf("[warning] %v (%s)", err, a.AlertId)
                    }
                case "annotations":
                    if err := json.Unmarshal(value, &a.Annotations); err != nil {
                        log.Printf("[warning] %v (%s)", err, a.AlertId)
                    }
                case "generator_url":
                    a.GeneratorURL = string(value)
            }
        }

        result = append(result, a) 
    }

    return result, nil
}

func (db *Client) SaveAlerts(alerts map[string]cache.Alert) error {

    stmt, err := db.client.Prepare("replace into alerts values (?,?,?,?,?,?,?,?,?,?,?)")
    if err != nil {
        return err
    }
    defer stmt.Close()

    for _, i := range alerts {

        labels, err := json.Marshal(i.Labels)
        if err != nil {
            return err
        }

        annotations, err := json.Marshal(i.Annotations)
        if err != nil {
            return err
        }

        _, err = stmt.Exec(
            i.AlertId,
            i.GroupId,
            i.State,
            i.ActiveAt,
            i.StartsAt,
            i.EndsAt,
            i.Repeat,
            i.ChangeSt,
            labels,
            annotations,
            i.GeneratorURL,
        )
        if err != nil {
            return err
        }

    }

    return nil
}

func (db *Client) AddAlert(alert cache.Alert) error {

    stmt, err := db.client.Prepare("insert into alerts values (?,?,?,?,?,?,?,?,?,?)")
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
        alert.State,
        alert.StartsAt,
        alert.EndsAt,
        alert.Repeat,
        alert.ChangeSt,
        labels,
        annotations,
        alert.GeneratorURL,
    )
    if err != nil {
        return err
    }

    return nil
}

func (db *Client) UpdAlert(alert cache.Alert) error {

    stmt, err := db.client.Prepare("update alerts set state=?,ends_at=?,repeat=?,change_st=?,annotations=?,generator_url=? where alert_id = ?")
    if err != nil {
        return err
    }
    defer stmt.Close()

    annotations, err := json.Marshal(alert.Annotations)
    if err != nil {
        return err
    }

    _, err = stmt.Exec(
        alert.State,
        alert.EndsAt,
        alert.Repeat,
        alert.ChangeSt,
        annotations,
        alert.GeneratorURL,
        alert.AlertId,
    )
    if err != nil {
        return err
    }

    return nil
}

func (db *Client) DeleteOldAlerts() (int64, error) {

    stmt, err := db.client.Prepare("delete from alerts where ends_at < UNIX_TIMESTAMP() - 86400 * ?")
    if err != nil {
        return 0, err
    }
    defer stmt.Close()

    res, err := stmt.Exec(db.config.HistoryDays)
    if err != nil {
        return 0, err
    }

    cnt, err := res.RowsAffected()
    if err != nil {
        return 0, err
    }

    return cnt, nil
}

func (db *Client) SaveProxyLog(proxy config.Proxy) error {
    stmt, err := db.client.Prepare("insert into proxy_logs (login,method,url,path,timestamp) values (?,?,?,?,?)")
    if err != nil {
        return err
    }
    defer stmt.Close()

    _, err = stmt.Exec(proxy.Login, proxy.Method, proxy.Url, proxy.Path, proxy.Timestamp)
    if err != nil {
        return err
    }

    return nil
}

func (db *Client) DeleteOldProxyLogs() (int64, error) {

    stmt, err := db.client.Prepare("delete from proxy_logs where timestamp < UNIX_TIMESTAMP() - 86400 * ?")
    if err != nil {
        return 0, err
    }
    defer stmt.Close()

    res, err := stmt.Exec(db.config.HistoryDays)
    if err != nil {
        return 0, err
    }

    cnt, err := res.RowsAffected()
    if err != nil {
        return 0, err
    }

    return cnt, nil
}