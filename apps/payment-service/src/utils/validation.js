const { z } = require('zod');

const paymentSchema = z.object({
  order_id: z.string().uuid({ message: "Geçersiz Sipariş ID formatı" }),
  customer_id: z.string().min(1, "Müşteri ID boş olamaz"),
  total_amount: z.number().positive("Tutar 0'dan büyük olmalı"),
});

module.exports = { paymentSchema };
