from flask import Flask, jsonify, request, send_from_directory
from flask_cors import CORS
import routeros_api
import sqlite3
import os
import time
from datetime import datetime, timedelta
from apscheduler.schedulers.background import BackgroundScheduler

app = Flask(__name__, static_folder='frontend', static_url_path='/')
CORS(app, resources={r"/*": {"origins": "*"}})

@app.route('/')
def serve_index():
    return send_from_directory('frontend', 'index.html')

DB_FILE = 'mikromon.db'
BACKUP_DIR = '/home/sairo/Antigravity/backups'

if not os.path.exists(BACKUP_DIR):
    os.makedirs(BACKUP_DIR)

def init_db():
    conn = sqlite3.connect(DB_FILE)
    c = conn.cursor()
    c.execute('''CREATE TABLE IF NOT EXISTS settings
                 (key TEXT PRIMARY KEY, value TEXT)''')
    c.execute('''CREATE TABLE IF NOT EXISTS interfaces
                 (name TEXT PRIMARY KEY, link_speed INTEGER, is_wan BOOLEAN, monitor_pppoe BOOLEAN)''')
    conn.commit()
    conn.close()

init_db()

def get_db_connection():
    conn = sqlite3.connect(DB_FILE)
    conn.row_factory = sqlite3.Row
    return conn

def get_creds():
    conn = get_db_connection()
    settings = dict(conn.execute('SELECT * FROM settings').fetchall())
    conn.close()
    if 'router_ip' not in settings:
        return None
    return settings

def connect_to_router(ip, user, password):
    connection = routeros_api.RouterOsApiPool(ip, username=user, password=password, plaintext_login=True)
    return connection.get_api(), connection

# --- Backup & Pruning Logic ---

def prune_backups():
    now = datetime.now()
    files = sorted([f for f in os.listdir(BACKUP_DIR) if f.endswith('.backup')])
    
    kept_files = set()
    
    def get_time(fname):
        try:
            # Matches: auto_backup_YYYYMMDD_HHMMSS.backup OR manual_backup_YYYYMMDD_HHMMSS.backup OR backup_YYYYMMDD_HHMMSS.backup
            parts = fname.split('_')
            # If standard 3 parts: backup_DATE_TIME.backup
            if len(parts) == 3:
                ts_str = parts[1] + parts[2].split('.')[0]
            # If 4 parts: type_backup_DATE_TIME.backup
            elif len(parts) >= 4:
                ts_str = parts[2] + parts[3].split('.')[0]
            else: return None
            
            return datetime.strptime(ts_str, "%Y%m%d%H%M%S")
        except:
            return None

    auto_files = []
    manual_files = []

    for f in files:
        t = get_time(f)
        if t:
            if f.startswith('manual_'):
                manual_files.append({'name': f, 'time': t})
            else:
                # Treat 'auto_' and legacy 'backup_' as auto
                auto_files.append({'name': f, 'time': t})

    # --- PROCESS AUTO BACKUPS (Smart Retention) ---
    daily_buckets = {} 
    weekly_buckets = {} 

    for item in auto_files:
        age_days = (now - item['time']).days
        if age_days < 7:
            kept_files.add(item['name'])
            continue
        day_key = item['time'].strftime("%Y-%m-%d")
        if 7 <= age_days < 90:
            if day_key not in daily_buckets: daily_buckets[day_key] = []
            daily_buckets[day_key].append(item)
            continue
        if 90 <= age_days < 1095:
             if day_key not in daily_buckets: daily_buckets[day_key] = []
             daily_buckets[day_key].append(item)
             continue
        if age_days >= 1095:
             week_key = item['time'].strftime("%Y-%W")
             if week_key not in weekly_buckets: weekly_buckets[week_key] = []
             weekly_buckets[week_key].append(item)

    for day, items in daily_buckets.items():
        items.sort(key=lambda x: x['time'])
        age = (now - items[0]['time']).days
        if age < 90:
            kept_files.add(items[0]['name']) 
            if len(items) > 1: kept_files.add(items[-1]['name']) 
        else:
            kept_files.add(items[0]['name']) 

    for week, items in weekly_buckets.items():
        items.sort(key=lambda x: x['time'])
        kept_files.add(items[0]['name'])

    # --- PROCESS MANUAL BACKUPS (Simple 5 Year Retention) ---
    for item in manual_files:
        age_days = (now - item['time']).days
        # Keep for 5 years (approx 1825 days)
        if age_days <= (365 * 5):
            kept_files.add(item['name'])

    # DELETE unkept files
    for f in files:
        if f not in kept_files:
            try:
                os.remove(os.path.join(BACKUP_DIR, f))
                print(f"Pruned Backup: {f}")
            except: pass

# Scheduler Initialization
scheduler = BackgroundScheduler()

def execute_backup_logic(manual=False):
    creds = get_creds()
    if not creds: return {"success": False, "error": "Credenciais não encontradas"}
    
    try:
        api, connection = connect_to_router(creds['router_ip'], creds['router_user'], creds['router_pass'])
        
        ts = datetime.now().strftime("%Y%m%d_%H%M%S")
        prefix = "manual_backup" if manual else "auto_backup"
        backup_name = f"{prefix}_{ts}" 
        
        # Trigger
        api.get_binary_resource('/').call('system/backup/save', {'name': backup_name.encode()})
        time.sleep(2) 
        
        # Download
        files = api.get_resource('/file').get()
        target_file = next((f for f in files if f['name'] == f"{backup_name}.backup" or f['name'] == backup_name), None)
        
        if target_file:
            fname = target_file['name']
            local_path = os.path.join(BACKUP_DIR, fname)
            
            # Using sshpass
            cmd = f"sshpass -p '{creds['router_pass']}' scp -o StrictHostKeyChecking=no {creds['router_user']}@{creds['router_ip']}:/{fname} {local_path}"
            ret = os.system(cmd)
            
            if ret != 0:
                connection.disconnect()
                return {"success": False, "error": "Falha no Download (SCP) - Verifique se o SSH está ativo na porta 22"}

            api.get_resource('/file').remove(id=target_file['id'])
            prune_backups()
            connection.disconnect()
            return {"success": True, "file": fname}
        
        connection.disconnect()
        return {"success": False, "error": "Arquivo não encontrado no Router"}
        
    except Exception as e:
        return {"success": False, "error": str(e)}

def run_backup_job():
    print("Starting Scheduled Backup...")
    res = execute_backup_logic(manual=False)
    print(f"Backup Result: {res}")

scheduler.add_job(run_backup_job, 'interval', minutes=60, id='backup_job')
scheduler.start()

# --- Routes ---

@app.route('/backups/run', methods=['POST'])
def manual_backup():
    result = execute_backup_logic(manual=True)
    if result['success']:
        return jsonify(result)
    else:
        return jsonify(result), 500

@app.route('/settings/verify', methods=['GET'])
def check_config():
    creds = get_creds()
    return jsonify({"configured": creds is not None})

@app.route('/setup/connect', methods=['POST'])
def setup_connect():
    data = request.json
    try:
        api, connection = connect_to_router(data['ip'], data['user'], data['password'])
        interfaces = api.get_resource('/interface').get()
        clean_interfaces = []
        for iface in interfaces:
            if iface.get('type') in ['ether', 'vlan', 'bridge']:
                clean_interfaces.append({'name': iface.get('name')})
        connection.disconnect()
        return jsonify({"success": True, "interfaces": clean_interfaces})
    except Exception as e:
        return jsonify({"success": False, "error": str(e)}), 400

@app.route('/setup/save', methods=['POST'])
def setup_save():
    data = request.json
    conn = get_db_connection()
    c = conn.cursor()
    c.execute('INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)', ('router_ip', data['ip']))
    c.execute('INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)', ('router_user', data['user']))
    c.execute('INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)', ('router_pass', data['password']))
    c.execute('INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)', ('router_name', data['name']))
    c.execute('DELETE FROM interfaces')
    for iface in data['interfaces']:
        c.execute('INSERT INTO interfaces (name, link_speed, is_wan, monitor_pppoe) VALUES (?, ?, ?, ?)',
                  (iface['name'], iface['speed'], iface['is_wan'], iface['monitor_pppoe']))
    conn.commit()
    conn.close()
    
    if scheduler.get_job('backup_job'): scheduler.remove_job('backup_job')
    scheduler.add_job(run_backup_job, 'interval', minutes=60, id='backup_job')
    
    return jsonify({"success": True})

@app.route('/backups/list')
def list_backups():
    files = sorted([f for f in os.listdir(BACKUP_DIR) if f.endswith('.backup')], reverse=True)
    backups = []
    for f in files:
        path = os.path.join(BACKUP_DIR, f)
        size_mb = os.path.getsize(path) / 1024 / 1024
        try:
            parts = f.split('_')
            if len(parts) >= 4: # type_backup_DATE_TIME
                ts_str = parts[2] + parts[3].split('.')[0]
            else: # backup_DATE_TIME (legacy)
                ts_str = parts[1] + parts[2].split('.')[0]
            
            dt = datetime.strptime(ts_str, "%Y%m%d%H%M%S")
            date_fmt = dt.strftime("%d/%m/%Y %H:%M")
        except: date_fmt = f
        
        btype = 'MANUAL' if f.startswith('manual_') else 'AUTO'

        backups.append({
            "filename": f,
            "date": date_fmt,
            "size": f"{size_mb:.2f} MB",
            "type": btype
        })
    return jsonify(backups)

@app.route('/backups/download/<path:filename>')
def download_backup(filename):
    return send_from_directory(BACKUP_DIR, filename, as_attachment=True)

@app.route('/stats')
def stats():
    creds = get_creds()
    if not creds: return jsonify({"error": "Not Configured"}), 503

    try:
        api, connection = connect_to_router(creds['router_ip'], creds['router_user'], creds['router_pass'])
        resource = api.get_resource('/system/resource').get()[0]
        
        conn = get_db_connection()
        configured_ifaces = conn.execute('SELECT * FROM interfaces').fetchall()
        conn.close()

        wan_stats = {"rx": 0, "tx": 0}
        for iface in configured_ifaces:
            if iface['is_wan']:
                traffic = api.get_binary_resource('/').call('interface/monitor-traffic', {'interface': iface['name'].encode(), 'once': 'true'.encode()})
                if traffic:
                    wan_stats['rx'] += int(traffic[0].get('rx-bits-per-second', 0))
                    wan_stats['tx'] += int(traffic[0].get('tx-bits-per-second', 0))
        
        pppoe_count = 0
        try:
            active_connections = api.get_resource('/ppp/active').get()
            pppoe_count = len([c for c in active_connections if c.get('service') == 'pppoe'])
        except: pppoe_count = 0

        local_backups = sorted([f for f in os.listdir(BACKUP_DIR) if f.endswith('.backup')])
        last_backup = "Nenhum"
        if local_backups:
             try:
                 # Try to parse the last one
                 fname = local_backups[-1]
                 parts = fname.split('_')
                 if len(parts) >= 4: ts_str = parts[2] + parts[3].split('.')[0]
                 else: ts_str = parts[1] + parts[2].split('.')[0]
                 
                 dt = datetime.strptime(ts_str, "%Y%m%d%H%M%S")
                 last_backup = dt.strftime("%d/%m/%Y %H:%M")
             except: last_backup = "Local"

        connection.disconnect()
        
        next_run = None
        job = scheduler.get_job('backup_job')
        if job and job.next_run_time:
            next_run = job.next_run_time.isoformat()

        return jsonify({
            "router_name": creds['router_name'],
            "cpu": resource.get('cpu-load'),
            "memory": resource.get('free-memory'),
            "uptime": resource.get('uptime'),
            "pppoe_active": pppoe_count,
            "rx_bps": wan_stats['rx'],
            "tx_bps": wan_stats['tx'],
            "temp": "N/A",
            "last_backup": last_backup,
            "next_backup": next_run
        })
    except Exception as e:
        import traceback
        traceback.print_exc()
        return jsonify({"error": str(e)}), 500

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)
