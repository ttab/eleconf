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

  time_expression {
    expression = ".meta(type='core/event').data{start, end}"
  }
}

document "core/organiser" {
  statuses = [
    "usable",
  ]
}
