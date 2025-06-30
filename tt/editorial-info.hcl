document "core/editorial-info" {
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

document "tt/editorial-info-type" {
  statuses = [
    "usable",
  ]
}
