from pydantic import BaseModel, Field
from typing import List, Optional
from datetime import datetime

class OrderItem(BaseModel):
    product_id: str
    quantity: int
    price: float

class OrderCreate(BaseModel):
    customer_id: str
    items: List[OrderItem]
    total_amount: float
    fast_delivery: bool = False 

class OrderResponse(OrderCreate):
    order_id: str
    status: str
    created_at: str