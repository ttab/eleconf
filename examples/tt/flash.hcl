document "core/flash" {
  statuses = [
    "draft",
    "done",
    "approved",
    "withheld",
    "cancelled",
    "usable",
  ]

  workflow = {
    step_zero  = "draft"
    checkpoint = "usable"
    negative_checkpoint = "unpublished"
    steps      = [
      "draft",
      "done",
      "approved",
      "withheld",
      "cancelled",
    ]
  }
}
