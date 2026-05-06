import jwt
import datetime

SECRET = "dev-secret-keep-it-safe"

def generate_token(email, role, days=7):
    payload = {
        "email": email,
        "role": role,
        "exp": datetime.datetime.now(datetime.UTC) + datetime.timedelta(days=days)
    }
    return jwt.encode(payload, SECRET, algorithm="HS256")

if __name__ == "__main__":
    print(f"Super Admin: {generate_token('admin@test.com', 'Super Admin')}")
    print(f"Site Admin: {generate_token('site@test.com', 'Site Admin')}")
    print(f"Viewer: {generate_token('viewer@test.com', 'Viewer')}")
