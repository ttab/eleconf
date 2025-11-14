document "tt/print-article" {
  statuses = [
    "draft",
    "usable",
    "needs_proofreading",
    "print_done",
    "cancelled",
  ]

  workflow = {
    step_zero  = "draft"
    checkpoint = "usable"
    negative_checkpoint = "unpublished"
    steps = [
      "draft",
      "print_done",
      "needs_proofreading",
      "cancelled",
    ]
  }
}

document "tt/print-layout" {
  statuses = [
    "usable",
  ]

  attachment "layout" {
    required       = true
    match_mimetype = [
      "application/vnd.scribus",
    ]
  }
}

document "tt/print-flow" {
  statuses = [
    "usable",
  ]
}

document "tt/tv-channel" {
  statuses = [
    "usable",
  ]

  attachment "logo" {
    required       = true
    match_mimetype = [
      "application/pdf",
    ]
  }
}
