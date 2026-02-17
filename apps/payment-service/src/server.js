const express = require('express');
const bodyParser = require('body-parser');
const { v4: uuidv4 } = require('uuid');

const logger = require('./utils/logger');
const { paymentSchema } = require('./utils/validation');
const { checkIdempotency } = require('./utils/idempotency');

const app = express();
const PORT = process.env.APP_PORT || 5000;

app.use(bodyParser.json({ type: 'application/*+json' }));
app.use(bodyParser.json());

app.get('/dapr/subscribe', (_req, res) => {
    res.json([
        {
            pubsubname: "order-pubsub",
            topic: "order_created",
            route: "process-payment"
        }
    ]);
});

async function callBankApi(amount, currency = "TRY") {
    return new Promise((resolve, reject) => {
        const latency = Math.floor(Math.random() * 400) + 100;
        
        setTimeout(() => {
            if (Math.random() < 0.1) {
                reject(new Error("BANK_CONNECTION_TIMEOUT"));
            } else {
                resolve({ transaction_id: uuidv4(), status: "SUCCESS" });
            }
        }, latency);
    });
}

app.post('/process-payment', async (req, res) => {
    try {
        const eventData = req.body.data;
        logger.info("📨 Event Alındı", { type: "ORDER_CREATED", raw_data: eventData });

        const validation = paymentSchema.safeParse(eventData);
        if (!validation.success) {
            logger.error("❌ Geçersiz Veri Formatı", { errors: validation.error.format() });
            return res.status(200).json({ status: "DROPPED_INVALID_DATA" });
        }

        const { order_id, total_amount, customer_id } = validation.data;

        if (!checkIdempotency(order_id)) {
            return res.status(200).json({ status: "ALREADY_PROCESSED" });
        }

        logger.info(`🔄 Ödeme Başlatılıyor...`, { order_id, amount: total_amount });

        if (total_amount > 50000) {
            logger.warn("⛔ Ödeme Reddedildi: Limit Aşımı", { order_id, limit: 50000, requested: total_amount });
            return res.status(200).json({ status: "DECLINED_LIMIT" });
        }

        await callBankApi(total_amount);
        
        logger.info("✅ Ödeme Başarıyla Alındı", { order_id, customer_id, amount: total_amount });
        
        return res.status(200).json({ status: "SUCCESS" });

    } catch (error) {
        if (error.message === "BANK_CONNECTION_TIMEOUT") {
            logger.error("🔥 Banka Erişim Hatası (Retry Yapılacak)", { error: error.message });
            return res.status(500).json({ error: "BANK_UNAVAILABLE" });
        }

        logger.error("💀 Beklenmeyen Hata", { error: error.message, stack: error.stack });
        return res.status(200).json({ status: "FAILED_UNKNOWN" });
    }
});

app.listen(PORT, () => {
    logger.info(`🚀 Payment Service (Node.js) Başlatıldı`, { port: PORT });
});
