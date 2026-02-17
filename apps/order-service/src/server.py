from sanic import Sanic
from sanic.response import json, text
from prometheus_client import generate_latest, CONTENT_TYPE_LATEST, Counter
from src.routes import order_bp

app = Sanic("OrderService")

ORDER_COUNTER = Counter('orders_created_total', 'Total number of created orders')

app.blueprint(order_bp)

@app.route("/health")
async def health_check(request):
    return json({"status": "UP"})

@app.route("/metrics")
async def metrics(request):
    return text(generate_latest().decode(), content_type=CONTENT_TYPE_LATEST)

if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8000)