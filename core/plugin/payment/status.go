package payment

// StatusPending marks a newly created payment order.
const StatusPending = "pending"

// StatusSuccess marks a successfully paid order.
const StatusSuccess = "success"

// StatusFailed marks a failed payment.
const StatusFailed = "failed"

// StatusExpired marks an expired unpaid order.
const StatusExpired = "expired"

// StatusCancelled marks a cancelled order.
const StatusCancelled = "cancelled"
