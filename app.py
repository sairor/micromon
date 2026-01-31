from flask import Flask, jsonify, request, send_from_directory, redirect, url_for
from flask_cors import CORS
from flask_login import LoginManager, UserMixin, login_user, login_required, logout_user, current_user
from werkzeug.security import generate_password_hash, check_password_hash
import routeros_api
import sqlite3
import os
import time
from datetime import datetime, timedelta
from apscheduler.schedulers.background import BackgroundScheduler

app = Flask(__name__, static_folder='frontend', static_url_path='/')
app.secret_key = 'supersecretkey_change_in_production' 
CORS(app, resources={r"/*": {"origins": "*"}})

login_manager = LoginManager()
login_manager.init_app(app)

@login_manager.unauthorized_handler
def unauthorized():
    return jsonify({'error': 'Unauthorized', 'authenticated': False}), 401

DB_FILE = 'mikromon.db'
BACKUP_BASE_DIR = '/home/sairo/Antigravity/backups'

if not os.path.exists(BACKUP_BASE_DIR):
    os.makedirs(BACKUP_BASE_DIR)

# --- User Model ---
class User(UserMixin):
    def __init__(self, id, username):
        self.id = id
        self.username = username

def get_db_connection():
    conn = sqlite3.connect(DB_FILE)
    conn.row_factory = sqlite3.Row
    return conn

@login_manager.user_loader
def load_user(user_id):
    conn = get_db_connection()
    user = conn.execute('SELECT * FROM users WHERE id = ?', (user_id,)).fetchone()
    conn.close()
    if user:
        return User(user['id'], user['username'])
    return None

def init_db():
    conn = sqlite3.connect(DB_FILE)
    c = conn.cursor()
    
    # Users
    c.execute('''CREATE TABLE IF NOT EXISTS users
                 (id INTEGER PRIMARY KEY AUTOINCREMENT,
                  username TEXT UNIQUE NOT NULL,
                  password_hash TEXT NOT NULL)''')

    # Routers
    c.execute('''CREATE TABLE IF NOT EXISTS routers
                 (id INTEGER PRIMARY KEY AUTOINCREMENT,
                  user_id INTEGER,
                  name TEXT,
                  host TEXT,
                  username TEXT,
                  password TEXT,
                  FOREIGN KEY(user_id) REFERENCES users(id))''')

    try:
        c.execute("ALTER TABLE interfaces ADD COLUMN router_id INTEGER DEFAULT 1")
    except: pass

    # Interfaces
    c.execute('''CREATE TABLE IF NOT EXISTS interfaces
                 (name TEXT, link_speed INTEGER, is_wan BOOLEAN, monitor_pppoe BOOLEAN, router_id INTEGER,
                  PRIMARY KEY (name, router_id))''')

    conn.commit()
    conn.close()

init_db()

# --- Auth Routes ---
@app.route('/register', methods=['POST'])
def register():
    data = request.json
    if not data or not data.get('username') or not data.get('password'):
        return jsonify({'error': 'Missing credentials'}), 400
    
    hashed = generate_password_hash(data['password'])
    conn = get_db_connection()
    try:
        conn.execute('INSERT INTO users (username, password_hash) VALUES (?, ?)',
                     (data['username'], hashed))
        conn.commit()
        return jsonify({'success': True})
    except sqlite3.IntegrityError:
        return jsonify({'error': 'Username taken'}), 409
    finally:
        conn.close()

@app.route('/login', methods=['POST'])
def login():
    data = request.json
    conn = get_db_connection()
    user = conn.execute('SELECT * FROM users WHERE username = ?', (data.get('username'),)).fetchone()
    conn.close()
    
    if user and check_password_hash(user['password_hash'], data.get('password')):
        user_obj = User(user['id'], user['username'])
        login_user(user_obj)
        return jsonify({'success': True, 'username': user['username']})
    
    return jsonify({'error': 'Invalid credentials'}), 401

@app.route('/logout', methods=['POST'])
@login_required
def logout():
    logout_user()
    return jsonify({'success': True})

@app.route('/auth/status')
def auth_status():
    if current_user.is_authenticated:
        return jsonify({'authenticated': True, 'username': current_user.username})
    return jsonify({'authenticated': False})

# --- Router Management ---
def get_router_or_404(router_id):
    conn = get_db_connection()
    router = conn.execute('SELECT * FROM routers WHERE id = ? AND user_id = ?', 
                          (router_id, current_user.id)).fetchone()
    conn.close()
    return router

@app.route('/routers', methods=['GET'])
@login_required
def list_routers():
    conn = get_db_connection()
    routers = conn.execute('SELECT id, name, host FROM routers WHERE user_id = ?', (current_user.id,)).fetchall()
    conn.close()
    return jsonify([dict(r) for r in routers])

@app.route('/routers/add', methods=['POST'])
@login_required
def add_router():
    data = request.json
    conn = get_db_connection()
    
    # Insert Router
    cursor = conn.cursor()
    cursor.execute('INSERT INTO routers (user_id, name, host, username, password) VALUES (?, ?, ?, ?, ?)',
                 (current_user.id, data['name'], data['ip'], data['user'], data['password']))
    router_id = cursor.lastrowid
    
    # Insert Interface
    if 'interfaces' in data:
        for iface in data['interfaces']:
             cursor.execute("INSERT OR REPLACE INTO interfaces (name, link_speed, is_wan, monitor_pppoe, router_id) VALUES (?, ?, ?, ?, ?)",
                  (iface['name'], iface['speed'], iface['is_wan'], iface['monitor_pppoe'], router_id))

    conn.commit()
    conn.close()
    return jsonify({'success': True})

@app.route('/setup/connect', methods=['POST'])
@login_required
def try_connect():
    data = request.json
    try:
        connection = routeros_api.RouterOsApiPool(data['ip'], username=data['user'], password=data['password'], plaintext_login=True)
        api = connection.get_api()
        interfaces = api.get_resource('/interface').get()
        connection.disconnect()
        
        iface_list = [{'name': i['name'], 'type': i.get('type', 'ether')} for i in interfaces]
        return jsonify({"success": True, "interfaces": iface_list})
    except Exception as e:
        return jsonify({"success": False, "error": str(e)})

def connect_to_router_by_id(router_id, user_id=None): # user_id optional to allow scheduler access
    conn = get_db_connection()
    query = 'SELECT * FROM routers WHERE id = ?'
    args = [router_id]
    if user_id: 
        query += ' AND user_id = ?'
        args.append(user_id)
        
    router = conn.execute(query, tuple(args)).fetchone()
    conn.close()
    
    if not router: return None, None
    connection = routeros_api.RouterOsApiPool(router['host'], username=router['username'], password=router['password'], plaintext_login=True)
    return connection.get_api(), connection

# --- Backup Logic ---
def get_router_backup_dir(router_id):
    path = os.path.join(BACKUP_BASE_DIR, str(router_id))
    if not os.path.exists(path): os.makedirs(path)
    return path

def prune_backups(router_id):
    backup_dir = get_router_backup_dir(router_id)
    now = datetime.now()
    try: files = sorted([f for f in os.listdir(backup_dir) if f.endswith('.backup')])
    except: return

    kept_files = set()
    
    def get_time(fname):
        try:
            parts = fname.split('_')
            if len(parts) == 3: ts_str = parts[1] + parts[2].split('.')[0]
            elif len(parts) >= 4: ts_str = parts[2] + parts[3].split('.')[0]
            else: return None
            return datetime.strptime(ts_str, "%Y%m%d%H%M%S")
        except: return None

    auto_files = []
    manual_files = []

    for f in files:
        t = get_time(f)
        if t:
            if 'manual_' in f: manual_files.append({'name': f, 'time': t})
            else: auto_files.append({'name': f, 'time': t})

    # Manual: 5 Years
    five_years_ago = now - timedelta(days=365*5)
    for item in manual_files:
        if item['time'] > five_years_ago: kept_files.add(item['name'])

    # Auto: Smart Retention
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
        elif age_days >= 90:
            if age_days < 1095:
                 if day_key not in daily_buckets: daily_buckets[day_key] = []
                 daily_buckets[day_key].append(item)
            else:
                 week_key = item['time'].strftime("%Y-%W")
                 if week_key not in weekly_buckets: weekly_buckets[week_key] = []
                 weekly_buckets[week_key].append(item)

    # Buckets Logic
    for bucket in [daily_buckets, weekly_buckets]:
        for key, items in bucket.items():
            items.sort(key=lambda x: x['time'])
            if items: kept_files.add(items[0]['name'])
            if len(items) > 1: kept_files.add(items[-1]['name'])

    for f in files:
        if f not in kept_files:
            try: os.remove(os.path.join(backup_dir, f))
            except: pass

def run_backup_job_for_router(router_id, router_data):
    try:
        connection = routeros_api.RouterOsApiPool(router_data['host'], username=router_data['username'], password=router_data['password'], plaintext_login=True)
        api = connection.get_api()
        
        filename = f"auto_backup_{datetime.now().strftime('%Y%m%d_%H%M%S')}.backup"
        api.get_binary_resource('/').call('system/backup/save', {'name': filename})
        time.sleep(2)

        local_path = os.path.join(get_router_backup_dir(router_id), filename)
        ssh_cmd = f"sshpass -p '{router_data['password']}' scp -o StrictHostKeyChecking=no {router_data['username']}@{router_data['host']}:/{filename} {local_path}"
        
        if os.system(ssh_cmd) == 0:
            del_cmd = f"sshpass -p '{router_data['password']}' ssh -o StrictHostKeyChecking=no {router_data['username']}@{router_data['host']} '/file remove {filename}'"
            os.system(del_cmd)
            prune_backups(router_id)
        
        connection.disconnect()
    except Exception as e:
        print(f"Backup failed for router {router_id}: {e}")

def run_global_backup_job():
    with app.app_context():
        conn = get_db_connection()
        routers = conn.execute('SELECT * FROM routers').fetchall()
        conn.close()
        for r in routers:
            run_backup_job_for_router(r['id'], r)

scheduler = BackgroundScheduler()
scheduler.add_job(run_global_backup_job, 'interval', minutes=60)
scheduler.start()

# --- Main API ---
@app.route('/stats')
@login_required
def stats():
    router_id = request.args.get('router_id')
    if not router_id: return jsonify({'error': 'Router ID required'}), 400
    
    # Ownership Check
    router = get_router_or_404(router_id)
    if not router: return jsonify({'error': 'Router not found'}), 404

    try:
        api, connection = connect_to_router_by_id(router_id, current_user.id)
        resource = api.get_resource('/system/resource').get()[0]
        try: pppoe = api.get_resource('/interface/pppoe-server/active-conn').get()
        except: pppoe = []
        
        # Traffic
        conn = get_db_connection()
        iface = conn.execute('SELECT * FROM interfaces WHERE router_id = ? AND is_wan = 1', (router_id,)).fetchone()
        conn.close()
        
        rx = 0
        tx = 0
        if iface:
            try:
                traffic = api.get_binary_resource('/').call('interface/monitor-traffic', {'interface': iface['name'], 'once': 'true'})
                if traffic:
                    rx = int(traffic[0].get('rx-bits-per-second', 0))
                    tx = int(traffic[0].get('tx-bits-per-second', 0))
            except: pass

        # Temperature
        temp = "N/A"
        try:
            health = api.get_resource('/system/health').get()
            for h in health:
                if 'name' in h and h['name'] == 'temperature':
                    temp = h['value']
                    break
                elif 'temperature' in h:
                    temp = h['temperature']
                    break
        except: pass

        connection.disconnect()

        # Backup Status
        backup_dir = get_router_backup_dir(router_id)
        backups = sorted([f for f in os.listdir(backup_dir) if f.endswith('.backup')])
        last_backup = "Nunca"
        if backups:
            ts = os.path.getmtime(os.path.join(backup_dir, backups[-1]))
            last_backup = datetime.fromtimestamp(ts).strftime('%d/%m/%Y %H:%M')

        # Next run is simply hour mark
        now = datetime.now()
        next_run = (now + timedelta(hours=1)).replace(minute=0, second=0, microsecond=0).isoformat()

        return jsonify({
            "router_name": router['name'],
            "cpu": resource.get('cpu-load'),
            "memory": resource.get('free-memory'),
            "uptime": resource.get('uptime'),
            "pppoe_active": len(pppoe),
            "temp": temp,
            "rx_bps": rx,
            "tx_bps": tx,
            "last_backup": last_backup,
            "next_backup": next_run
        })
    except Exception as e:
        return jsonify({"error": str(e)}), 500

@app.route('/backups/list')
@login_required
def list_backups():
    router_id = request.args.get('router_id')
    if not get_router_or_404(router_id): return jsonify([])

    backup_dir = get_router_backup_dir(router_id)
    files = []
    try:
        for f in os.listdir(backup_dir):
            if f.endswith('.backup'):
                path = os.path.join(backup_dir, f)
                size = os.path.getsize(path) / 1024 / 1024
                mod_time = datetime.fromtimestamp(os.path.getmtime(path)).strftime('%d/%m/%Y %H:%M')
                b_type = "MANUAL" if "manual_" in f else "AUTO"
                files.append({"filename": f, "date": mod_time, "size": f"{size:.2f} MB", "type": b_type})
    except: pass
    return jsonify(sorted(files, key=lambda x: x['date'], reverse=True))

@app.route('/backups/download/<router_id>/<filename>')
@login_required
def download_backup(router_id, filename):
    if not get_router_or_404(router_id): return "Unauthorized", 403
    return send_from_directory(get_router_backup_dir(router_id), filename)

@app.route('/backups/run', methods=['POST'])
@login_required
def manual_backup():
    router_id = request.json.get('router_id')
    router = get_router_or_404(router_id)
    if not router: return jsonify({'error': 'Router not found'}), 404

    try:
        api, connection = connect_to_router_by_id(router_id, current_user.id)
        filename = f"manual_backup_{datetime.now().strftime('%Y%m%d_%H%M%S')}.backup"
        api.get_binary_resource('/').call('system/backup/save', {'name': filename})
        time.sleep(2)
        
        local_path = os.path.join(get_router_backup_dir(router_id), filename)
        ssh_cmd = f"sshpass -p '{router['password']}' scp -o StrictHostKeyChecking=no {router['username']}@{router['host']}:/{filename} {local_path}"
        
        if os.system(ssh_cmd) == 0:
             del_cmd = f"sshpass -p '{router['password']}' ssh -o StrictHostKeyChecking=no {router['username']}@{router['host']} '/file remove {filename}'"
             os.system(del_cmd)
             prune_backups(router_id)
             connection.disconnect()
             return jsonify({'success': True})
        else:
             connection.disconnect()
             return jsonify({'error': 'Download Failed (SSH)'}), 500
    except Exception as e:
        return jsonify({'error': str(e)}), 500

@app.route('/')
def serve_index():
    return send_from_directory('frontend', 'index.html')

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)
