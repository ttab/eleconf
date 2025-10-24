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

  time_expression {
    expression = ".meta(type='core/planning-item').data{start_date:date, end_date:date, tz=date_tz?}"
  }

  time_expression {
    expression = ".meta(type='core/assignment').data{start_date:date, end_date:date, tz=date_tz?}"
  }

  time_expression {
    expression = ".meta(type='core/assignment').data{start?, publish?}"
  }
}
