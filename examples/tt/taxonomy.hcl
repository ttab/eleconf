document "core/place" {
  statuses = ["usable"]
}

document "core/person" {
  statuses = ["usable"]
}

document "core/story" {
  statuses = ["usable"]
}

document "core/channel" {
  statuses = ["usable"]

  bounded_collection = true
}

document "core/section" {
  statuses = ["usable"]

  bounded_collection = true
}

document "core/organisation" {
  statuses = ["usable"]
}

document "core/organiser" {
  statuses = ["usable"]
}

document "core/category" {
  statuses = ["usable"]

  bounded_collection = true
}
