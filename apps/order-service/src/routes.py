from sanic import Blueprint
from sanic.response import json
from dapr.clients import DaprClient
from src.models import OrderCreate, OrderResponse
import json as pyjson
import uuid
from datetime import datetime
import asyncio
import random

order_bp = Blueprint("orders", url_prefix="/api/v1/orders")

STATE_STORE_NAME = "statestore"
PUBSUB_NAME = "order-pubsub"

@order_bp.post("/")
async def create_order(request):
    try:
        order_in = OrderCreate(**request.json)
        
        order_id = str(uuid.uuid4())
        order_data = {
            "order_id": order_id,
            "status": "PENDING",
            "created_at": datetime.utcnow().isoformat(),
            **order_in.model_dump()
        }

        with DaprClient() as d:
            d.save_state(
                store_name=STATE_STORE_NAME,
                key=order_id,
                value=pyjson.dumps(order_data)
            )
            
            d.publish_event(
                pubsub_name=PUBSUB_NAME,
                topic_name="order_created",
                data=pyjson.dumps(order_data),
                data_content_type='application/json'
            )

        return json(order_data, status=201)
    
    except Exception as e:
        return json({"error": str(e)}, status=400)

@order_bp.get("/<order_id>")
async def get_order(request, order_id):
    with DaprClient() as d:
        state = d.get_state(store_name=STATE_STORE_NAME, key=order_id)
        
        if not state.data:
            return json({"error": "Order not found"}, status=404)
            
        return json(pyjson.loads(state.data))

@order_bp.post("/<order_id>/cancel")
async def cancel_order(request, order_id):
    with DaprClient() as d:
        state = d.get_state(store_name=STATE_STORE_NAME, key=order_id)
        
        if not state.data:
            return json({"error": "Order not found"}, status=404)
        
        order_data = pyjson.loads(state.data)
        
        if order_data["status"] == "SHIPPED":
            return json({"error": "Cannot cancel shipped order"}, status=400)
            
        order_data["status"] = "CANCELLED"
        
        d.save_state(
            store_name=STATE_STORE_NAME,
            key=order_id,
            value=pyjson.dumps(order_data)
        )
        
        return json({"message": "Order cancelled", "status": "CANCELLED"})

@order_bp.get("/search")
async def search_orders(request):
    customer_id = request.args.get("customer_id")
    if not customer_id:
        return json({"error": "customer_id required"}, status=400)

    query = {
        "filter": {
            "EQ": { "customer_id": customer_id }
        },
        "sort": [
            { "key": "created_at", "order": "DESC" }
        ]
    }

    try:
        with DaprClient() as d:
            resp = d.query_state(
                store_name=STATE_STORE_NAME,
                query=pyjson.dumps(query)
            )
            results = [pyjson.loads(item.value) for item in resp.results]
            return json({"count": len(results), "orders": results})
    except Exception as e:
        return json({"error": "Query not supported by underlying store", "details": str(e)}, status=501)

@order_bp.post("/simulate-failure")
async def chaos_endpoint(request):
    error_type = request.json.get("type", "latency")
    
    if error_type == "latency":
        await asyncio.sleep(random.uniform(2, 5))
        return json({"message": "I was slow but I finished"})
    
    elif error_type == "crash":
        return json({"error": "Simulated Crash!"}, status=500)
        
    return json({"message": "Specify type: latency or crash"})