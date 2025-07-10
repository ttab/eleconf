document "core/planning-item" {
  statuses = [
    "draft",
    "done",
    "cancelled",
    "usable",
  ]

  workflow = {
    step_zero  = "draft"
    checkpoint = "usable"
    negative_checkpoint = "unpublished"
    steps = [
      "draft",
      "done",
      "cancelled",
    ]
  }
}
