document "core/event" {
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
    steps      = [
      "draft",
      "done",
      "cancelled",
    ]
  }
}

document "core/organiser" {
  statuses = [
    "usable",
  ]
}
