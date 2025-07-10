document "core/article" {
  meta_doc = "core/article+meta"

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

document "core/author" {
  statuses = ["usable"]
}

document "core/factbox" {
  statuses = ["usable"]
}
