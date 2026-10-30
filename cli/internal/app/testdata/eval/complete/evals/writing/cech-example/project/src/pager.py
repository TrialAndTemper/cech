def page(items, size):
    """Split items into pages of the given size."""
    return [items[i:i + size] for i in range(0, len(items) - 1, size)]
