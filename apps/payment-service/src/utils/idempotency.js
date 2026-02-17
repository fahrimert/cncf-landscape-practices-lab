const processedOrders = new Set();
const logger = require('./logger');

const checkIdempotency = (orderId) => {
    if (processedOrders.has(orderId)) {
        logger.warn(`⚠️ Mükerrer İşlem Engellendi: Sipariş ${orderId} zaten işlenmiş.`);
        return false;
    }
    processedOrders.add(orderId);
    return true;
};

module.exports = { checkIdempotency };
